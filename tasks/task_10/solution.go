package main

import (
	"cmp"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Task struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	Done      bool      `json:"done"`
	UpdatedAt time.Time `json:"updatedAt"`
}
type TaskRepo interface {
	Create(title string) (Task, error)
	Get(id string) (Task, bool)
	List(done bool) []Task
	SetDone(id string, done bool) (Task, error)
}
type Clock interface{ Now() time.Time }

var (
	ErrNotFound     = errors.New("task not found")
	ErrInvalidTitle = errors.New("invalid title")
)

type inMemoryTaskRepo struct {
	mu    sync.RWMutex
	clock Clock
	seq   uint64
	tasks map[string]Task
}

func NewInMemoryTaskRepo(clock Clock) TaskRepo {
	return &inMemoryTaskRepo{clock: clock, tasks: make(map[string]Task)}
}

func (r *inMemoryTaskRepo) Create(title string) (Task, error) {
	title = strings.TrimSpace(title)
	if title == "" {
		return Task{}, ErrInvalidTitle
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	r.seq++
	task := Task{
		ID:        strconv.FormatUint(r.seq, 10),
		Title:     title,
		UpdatedAt: r.clock.Now(),
	}
	r.tasks[task.ID] = task
	return task, nil
}

func (r *inMemoryTaskRepo) Get(id string) (Task, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	task, ok := r.tasks[id]
	return task, ok
}

func (r *inMemoryTaskRepo) List(done bool) []Task {
	r.mu.RLock()
	defer r.mu.RUnlock()

	list := make([]Task, 0, len(r.tasks))
	for _, task := range r.tasks {
		if task.Done == done {
			list = append(list, task)
		}
	}
	return list
}

func (r *inMemoryTaskRepo) SetDone(id string, done bool) (Task, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	task, ok := r.tasks[id]
	if !ok {
		return Task{}, ErrNotFound
	}
	if task.Done == done {
		return task, nil
	}
	task.Done = done
	task.UpdatedAt = r.clock.Now()
	r.tasks[id] = task
	return task, nil
}

type httpHandler struct{ repo TaskRepo }

func NewHTTPHandler(repo TaskRepo) http.Handler {
	return &httpHandler{repo: repo}
}

func (h *httpHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/tasks" {
		switch r.Method {
		case http.MethodGet:
			h.handleList(w, r)
		case http.MethodPost:
			h.handleCreate(w, r)
		default:
			methodNotAllowed(w, http.MethodGet, http.MethodPost)
		}
		return
	}

	id, ok := strings.CutPrefix(r.URL.Path, "/tasks/")
	if !ok || id == "" || strings.Contains(id, "/") {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	switch r.Method {
	case http.MethodGet:
		h.handleGet(w, r, id)
	case http.MethodPatch:
		h.handlePatch(w, r, id)
	default:
		methodNotAllowed(w, http.MethodGet, http.MethodPatch)
	}
}

func (h *httpHandler) handleCreate(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Title *string `json:"title"`
	}
	if err := decodeStrictJSON(r.Body, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if req.Title == nil {
		writeError(w, http.StatusBadRequest, "title is required")
		return
	}

	task, err := h.repo.Create(*req.Title)
	switch {
	case errors.Is(err, ErrInvalidTitle):
		writeError(w, http.StatusBadRequest, err.Error())
	case err != nil:
		writeError(w, http.StatusInternalServerError, "internal error")
	default:
		writeJSON(w, http.StatusCreated, task)
	}
}

func (h *httpHandler) handleGet(w http.ResponseWriter, _ *http.Request, id string) {
	task, ok := h.repo.Get(id)
	if !ok {
		writeError(w, http.StatusNotFound, ErrNotFound.Error())
		return
	}
	writeJSON(w, http.StatusOK, task)
}

func (h *httpHandler) handleList(w http.ResponseWriter, r *http.Request) {
	done := false
	switch values := r.URL.Query()["done"]; {
	case len(values) == 0:
	case len(values) == 1 && values[0] == "true":
		done = true
	case len(values) == 1 && values[0] == "false":
	default:
		writeError(w, http.StatusBadRequest, `done must be "true" or "false"`)
		return
	}

	tasks := h.repo.List(done)
	if tasks == nil {
		tasks = []Task{}
	}
	slices.SortFunc(tasks, func(a, b Task) int {
		if c := b.UpdatedAt.Compare(a.UpdatedAt); c != 0 {
			return c
		}
		return cmp.Compare(a.ID, b.ID)
	})
	writeJSON(w, http.StatusOK, tasks)
}

func (h *httpHandler) handlePatch(w http.ResponseWriter, r *http.Request, id string) {
	var req struct {
		Done *bool `json:"done"`
	}
	if err := decodeStrictJSON(r.Body, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if req.Done == nil {
		writeError(w, http.StatusBadRequest, "done is required")
		return
	}

	task, err := h.repo.SetDone(id, *req.Done)
	switch {
	case errors.Is(err, ErrNotFound):
		writeError(w, http.StatusNotFound, err.Error())
	case err != nil:
		writeError(w, http.StatusInternalServerError, "internal error")
	default:
		writeJSON(w, http.StatusOK, task)
	}
}

func decodeStrictJSON(r io.Reader, v any) error {
	dec := json.NewDecoder(r)
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return err
	}
	if err := dec.Decode(&json.RawMessage{}); !errors.Is(err, io.EOF) {
		return errors.New("request body must contain a single JSON object")
	}
	return nil
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func methodNotAllowed(w http.ResponseWriter, allowed ...string) {
	w.Header().Set("Allow", strings.Join(allowed, ", "))
	writeError(w, http.StatusMethodNotAllowed, "method not allowed")
}
