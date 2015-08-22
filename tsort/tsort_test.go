package main

import (
	"bytes"
	"testing"
)

var (
	nums = map[string]string{
		`3 8
3 10
5 11
7 8
7 11
8 9
11 2
11 9
11 10`: `3
5
7
11
8
10
2
9
`, `8 9
1 4
1 2
4 2
4 3
3 2
5 2
3 5
8 2
8 6`: `1
8
4
6
9
3
5
2
`,
		`4 4
2 4
4 1
3 1`: `2
3
4
1
`}
)

func TestTsort(t *testing.T) {

	for unsorted, sorted := range nums {
		var buf bytes.Buffer

		buf.WriteString(unsorted)

		tsort(&buf)

		if buf.String() != sorted {
			t.Errorf("Got: %q\n\nWanted: %q", buf.String(), sorted)
		}
	}
}
