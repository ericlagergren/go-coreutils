// Copyright (c) 2014-2016 Eric Lagergren
// Use of this source code is governed by the GPL v3 or later.

package main

import (
	"bufio"
	"os"
	"syscall"

	"github.com/EricLagergren/go-coreutils/internal/flag"

	"golang.org/x/sys/unix"
)

func main() {
	var ok int // return status

	outStat, err := os.Stdout.Stat()
	if err != nil {
		fatal.Fatalln(err)
	}
	outReg := outStat.Mode().IsRegular()
	outBsize := int(outStat.Sys().(*syscall.Stat_t).Blksize)

	// catch (./cat) < /etc/group
	var args []string
	if flag.NArg() == 0 {
		args = []string{"-"}
	} else {
		args = flag.Args()
	}

	// the main loop
	var file *os.File
	for _, arg := range args {

		if arg == "-" {
			file = os.Stdin
		} else {
			file, err = os.Open(arg)
			if err != nil {
				fatal.Fatalln(err)
			}
		}

		inStat, err := file.Stat()
		if err != nil {
			fatal.Fatalln(err)
		}
		if inStat.IsDir() {
			fatal.Printf("%s: Is a directory\n", file.Name())
		}
		inBsize := int(inStat.Sys().(*syscall.Stat_t).Blksize)

		// prefetch! prefetch! prefetch!
		unix.Fadvise(int(file.Fd()), 0, 0, unix.FADV_SEQUENTIAL)

		// Make sure we're not catting a file to itself,
		// provided it's a regular file. Catting a non-reg
		// file to itself is cool.
		// e.g. cat file > file
		if outReg && os.SameFile(outStat, inStat) {
			if n, _ := file.Seek(0, os.SEEK_CUR); n < inStat.Size() {
				fatal.Fatalf("%s: input file is output file\n", file.Name())
			}
		}

		if simple {
			// Select larger block size
			size := max(inBsize, outBsize)
			outBuf := bufio.NewWriterSize(os.Stdout, size)
			ok ^= simpleCat(file, outBuf)

			// Flush because we don't have a chance to in
			// simpleCat() because we use io.Copy()
			outBuf.Flush()
		} else {
			// If you want to know why, exactly, I chose
			// outBsize -1 + inBsize*4 + 20, read GNU's cat
			// source code. The tl;dr is the 20 is the counter
			// buffer, inBsize*4 is from potentially prepending
			// the control characters (M-^), and outBsize is
			// due to new tests for newlines.
			size := outBsize - 1 + inBsize*4 + 20
			outBuf := bufio.NewWriterSize(os.Stdout, size)
			inBuf := make([]byte, inBsize+1)
			ok ^= cat(file, inBuf, outBuf)
		}

		file.Close()
	}

	os.Exit(ok)
}
