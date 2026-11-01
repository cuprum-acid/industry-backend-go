package main

func rotateRunes(s string, shift int) string {
	runes := []rune(s)
	n := len(runes)
	if n == 0 {
		return s
	}

	k := shift % n
	if k < 0 {
		k += n
	}

	out := make([]rune, 0, n)
	out = append(out, runes[k:]...)
	out = append(out, runes[:k]...)

	return string(out)
}
