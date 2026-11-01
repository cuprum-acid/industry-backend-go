package main

import (
	"container/list"
	"sync"
)

type LRU[K comparable, V any] interface {
	Get(key K) (value V, ok bool)
	Set(key K, value V)
}
type entry[K comparable, V any] struct {
	key   K
	value V
}
type LRUCache[K comparable, V any] struct {
	capacity int
	mu       sync.Mutex
	ll       list.List
	items    map[K]*list.Element
}

func NewLRUCache[K comparable, V any](capacity int) *LRUCache[K, V] {
	c := &LRUCache[K, V]{capacity: capacity}
	if capacity > 0 {
		c.items = make(map[K]*list.Element, capacity)
	}
	c.ll.Init()
	return c
}

func (c *LRUCache[K, V]) Get(key K) (value V, ok bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	el, ok := c.items[key]
	if !ok {
		return value, false
	}
	c.ll.MoveToFront(el)
	return el.Value.(*entry[K, V]).value, true
}

func (c *LRUCache[K, V]) Set(key K, value V) {
	if c.capacity <= 0 {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if el, ok := c.items[key]; ok {
		el.Value.(*entry[K, V]).value = value
		return
	}
	if c.ll.Len() >= c.capacity {
		oldest := c.ll.Back()
		c.ll.Remove(oldest)
		delete(c.items, oldest.Value.(*entry[K, V]).key)
	}
	c.items[key] = c.ll.PushFront(&entry[K, V]{key: key, value: value})
}
