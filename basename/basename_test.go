package main

import (
	"testing"
)

func TestBasename(t *testing.T) {

	cases := []struct {
		in   []string
		want string
	}{
		{[]string{"/usr/bin/sort"}, "sort\n"},
		{[]string{"include/stdio.h", ".h"}, "stdio\n"},
		{[]string{"-a", "any/str1", "any/str2"}, "str1\nstr2\n"},
		{[]string{"-s", ".h", "include/stdio.h"}, "stdio\n"},
		{[]string{"-s", ".h", "-a", "any/lib.h", "any/lib2.h"}, "str1\nstr2\n"},
		{[]string{"-z", "any/str1"}, "str1"},
		{[]string{"-z", "-a", "any/str1", "any/str2"}, "str1str2"},
		{[]string{"-z", "-s", ".h", "-a", "any/lib.h", "any/lib2.h"}, "liblib2"},
	}

	for _, c := range cases {
		got := basename(c.in)

		if got != c.want {
			t.Errorf("basename (%q) == %q, want %q", c.in, got, c.want)
		}
	}
}
