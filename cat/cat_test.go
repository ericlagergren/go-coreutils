// +build linux

package main

import (
	"bufio"
	"bytes"
	"io"
	"os"
	"os/exec"
	"syscall"
	"testing"
)

var flist = [...]string{
	"test_files/lang_ru.txt",
	"test_files/dict_en.txt",
	"test_files/spaces_en.txt",
	"test_files/coreutils_man_en.txt",
}

var buf bytes.Buffer

func TestCat(t *testing.T) {

	showNonPrinting = true
	*nonPrint = true
	*npEnds = true
	*npTabs = true

	for i, f := range flist {

		// set up our capture of stdout
		stdout := os.Stdout
		r, w, err := os.Pipe()
		if err != nil {
			t.Fatal(err)
		}
		os.Stdout = w

		file, err := os.Open(f)
		if err != nil {
			t.Error(err)
		}

		inStat, err := file.Stat()
		if err != nil {
			t.Error(err)
		}
		if inStat.IsDir() {
			t.Errorf("%s: is a directory\n", file.Name())
		}

		inBsize := int(inStat.Sys().(*syscall.Stat_t).Blksize)
		size := 20 + inBsize*4
		outBuf := bufio.NewWriterSize(os.Stdout, size)
		inBuf := make([]byte, inBsize+1)

		cat(file, inBuf, outBuf)
		file.Close()

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

		// now get stdout of native cat
		cat := exec.Command("cat", "-A", flist[i])

		b, err := cat.Output()
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

}
