package main

import "strings"

func greet(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		name = "World"
	}

	return "Hello, " + name + "!"
}
