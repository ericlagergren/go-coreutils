package main

import (
	"bytes"
	"io"
	"os"
	"os/exec"
	"testing"
)

var flist = []string{
	"_testdata/lang_ru.txt",
	"_testdata/dict_en.txt",
	"_testdata/spaces_en.txt",
	"_testdata/coreutils_man_en.txt",
}

var buf bytes.Buffer

func TestWC(t *testing.T) {

	fs := getFileStatus(4, flist)
	numberWidth = findNumberWidth(4, fs)

	*printLines = true
	*printWords = true
	*printChars = true
	*printBytes = true
	*printLineLength = true

	// set up our capture of stdout
	stdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w

	// now output stdout
	for i, file := range flist {
		wcFile(file, fs[i])
	}
	writeCounts(totalLines, totalWords,
		totalChars, totalBytes, maxLineLength, "total")

	// capture the stdout
	outC := make(chan string)
	go func() {
		var b bytes.Buffer
		_, err := io.Copy(&b, r)
		r.Close()
		if err != nil {
			t.Fatal(err)
		}
		outC <- b.String()
	}()

	// now get stdout of native wc
	wc := exec.Command("wc", "-lwmcL",
		flist[0], flist[1], flist[2], flist[3])

	b, err := wc.Output()
	if err != nil {
		t.Fatal(err)
	}
	buf.Write(b)

	// stop capturing of stdout
	w.Close()
	os.Stdout = stdout
	out := <-outC

	// check strings
	if out != buf.String() {
		t.Fatalf("Got:\n%s\n\nExpected:\n%s\n", out, buf.String())
	}

}
