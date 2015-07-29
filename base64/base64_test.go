package main

import (
	"log"
	"testing"
)

func TestBase64Decode(t *testing.T) {

	cases := []struct {
		in, want string
	}{
		{"aGVsbG8gd29ybGQK", "hello world\n"},
		{"cGxlYXNlLCBkZWNvZGUgbWUK", "please, decode me\n"},
	}

	for _, c := range cases {

		decodedBytes, err := base64Decode([]byte(c.in))
		if err != nil {
			log.Fatal(err)
		}
		got := string(decodedBytes)

		if got != c.want {
			t.Errorf("base64 (%q) == %q, want %q", c.in, got, c.want)
		}
	}
}

func TestBase64Encode(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"hello world\n", "aGVsbG8gd29ybGQK"},
		{"please, decode me\n", "cGxlYXNlLCBkZWNvZGUgbWUK"},
	}

	for _, c := range cases {

		encodedBytes := base64Encode([]byte(c.in))
		got := string(encodedBytes)

		if got != c.want {
			t.Errorf("base64 (%q) == %q, want %q", c.in, got, c.want)
		}
	}
}

func main() {}
