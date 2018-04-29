package main

import (
	"fmt"
	"os"
	"testing"
)

// Normal call with trailing new line
func TestEcho(t *testing.T) {
	// Fake arguments and reset after processing
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()
	os.Args = []string{"echo", "Hello", "World"}

	want := fmt.Sprintln("Hello World")
	got := echo()

	if got != want {
		t.Fatalf("expected: %v got: %v", want, got)
	}
}

// Without the trailing new line.
func TestEcho_n(t *testing.T) {
	// Fake arguments and reset after processing
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()
	os.Args = []string{"echo", "-n", "Hello", "World"}

	want := fmt.Sprint("Hello World")
	got := echo()

	if got != want {
		t.Fatalf("expected: %v got: %v", want, got)
	}
}
