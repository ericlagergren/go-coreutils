package main

import (
	"bytes"
	"crypto/rand"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"os/exec"
	"strings"
	"testing"
	"testing/quick"
)

var xxdFile = flag.String("xxdFile", "", "File to test against.")

func TestXXD(t *testing.T) {
	if *xxdFile == "" {
		t.Skip("-xxdFile argument not given")
	}
	data, err := ioutil.ReadFile(*xxdFile)
	if err != nil {
		t.Fatal(err)
	}
	test := func(fn func(r io.Reader, w io.Writer, s string) error) func(n uint64) []string {
		return func(n uint64) []string {
			size := n % uint64(len(data))
			fmt.Printf("%d\n", size)
			var out bytes.Buffer
			if err := fn(&pathologicalReader{data[0:size]}, &out, ""); err != nil {
				return []string{err.Error()}
			}
			return strings.Split(out.String(), "\n")
		}
	}
	if err := quick.CheckEqual(test(xxd), test(xxdNative), nil); err != nil {
		cErr := err.(*quick.CheckEqualError)
		size := cErr.In[0].(uint64) % uint64(len(data))
		for i := range cErr.Out1[0].([]string) {
			got := cErr.Out1[0].([]string)[i]
			want := cErr.Out2[0].([]string)[i]
			if got != want {
				t.Errorf("size: %d\n\ngot : %s\nwant: %s\n", size, got, want)
				break
			}
		}
	}
}

type pathologicalReader struct {
	data []byte
}

func (p *pathologicalReader) Read(b []byte) (int, error) {
	n := len(b)
	if n > len(p.data) {
		n = len(p.data)
	}
	if n > 1 {
		n--
	}
	copy(b, p.data[0:n])
	p.data = p.data[n:]
	if len(p.data) == 0 {
		return n, io.EOF
	}
	return n, nil
}

func BenchmarkXXD(b *testing.B) {
	b.StopTimer()
	data := make([]byte, b.N)
	if _, err := io.ReadFull(rand.Reader, data); err != nil {
		b.Fatal(err)
	}
	buf := bytes.NewBuffer(data)
	b.StartTimer()
	if err := xxd(buf, ioutil.Discard, ""); err != nil {
		b.Fatal(err)
	}
}

func xxdNative(r io.Reader, w io.Writer, s string) error {
	xxd := exec.Command("xxd.bak", "-")
	xxd.Stdin = r
	xxd.Stdout = w
	xxd.Stderr = w
	return xxd.Run()
}
