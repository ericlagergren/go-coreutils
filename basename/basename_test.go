package main

import (
	"bytes"
	"log"
	"os/exec"
	"testing"
)

func TestPerformBasename(t *testing.T) {

	cases := []struct {
		in   []string
		want string
	}{
		{[]string{"/usr/bin/sort", ""}, "sort\n"},
		{[]string{"include/stdio.h", ""}, "stdio.h\n"},
		{[]string{"include/stdio.h", ".h"}, "stdio\n"},
	}

	for _, c := range cases {
		got := performBasename(c.in[0], c.in[1], false)

		if got != c.want {
			t.Errorf("basename (%q) == %q, want %q", c.in, got, c.want)
		}
	}
}

func TestBasename(t *testing.T) {

	cases := []struct {
		in   []string
		want string
	}{
		{[]string{"-a", "any/str1 any/str2"}, "str1\nstr2\n"},
		{[]string{"-a", "any/str1 any/str2"}, "str1\nstr2\n"},
		{[]string{"-a", "/a//b //a/b"}, "b\nb\n"},
		{[]string{"-s", ".h", "include/stdio.h"}, "stdio\n"},
		{[]string{"-s", ".h", "-a", "any/lib.h any/lib2.h"}, "lib\nlib2\n"},
		{[]string{"-z", "any/str1"}, "str1"},
		{[]string{"-z", "-a", "any/str1 any/str2"}, "str1str2"},
		{[]string{"-z", "-s", ".h", "-a", "any/lib.h any/lib2.h"}, "liblib2"},
	}

	for _, c := range cases {
		var out bytes.Buffer
		cmd := exec.Command("./basename", c.in...)
		cmd.Stdout = &out
		err := cmd.Run()
		if err != nil {
			log.Fatal(err)
		}

		got := out.String()
		if got != c.want {
			t.Errorf("basename (%q) == %q, want %q", c.in, got, c.want)
		}
	}

}

func main() {}
