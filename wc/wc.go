// Copyright (c) 2015 Eric Lagergren
// Use of this source code is governed by the GPL v3 or later.

package main

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"unicode"

	"github.com/EricLagergren/go-coreutils/internal/flag"

	"github.com/EricLagergren/go-gnulib/sysinfo"
)

const (
	nullByte   = 0x00
	bufferSize = (64 * 1024)
)

var (
	// Our cumulative number of lines, words, chars, and bytes.
	totalLines    int64
	totalWords    int64
	totalChars    int64
	totalBytes    int64
	maxLineLength int64

	// For pretty printing.
	numberWidth int
	printOne    bool

	// For getFileStatus.
	errNoStat = errors.New("no stat")

	// Our cli args
	printLines      = flag.BoolP("lines", "l", false, "print the newline counts")
	printWords      = flag.BoolP("words", "w", false, "print the word counts")
	printChars      = flag.BoolP("chars", "m", false, "print the character counts")
	printBytes      = flag.BoolP("bytes", "c", false, "print the byte counts")
	printLineLength = flag.BoolP("max-line-length", "L", false, "print the length of the longest line")
	filesFrom       = flag.String("files0-from", "", `read input from the files specified by
                             NUL-terminated names in file F;
                             If F is - then read names from standard input`)
	tabWidth     = flag.Int64P("tab", "t", 8, "change the tab width")
	constVersion = flag.BoolP("unicode-version", "u", false, "display unicode version and exit")

	logger = log.New(os.Stderr, "", 0)
)

func fatal(format string, v ...interface{}) {
	logger.Fatalf("%s: %s\n", flag.Program, fmt.Sprintf(format, v...))
}

func print(format string, v ...interface{}) {
	logger.Printf("%s: %s\n", flag.Program, fmt.Sprintf(format, v...))
}

type fstatus struct {
	failed error
	stat   os.FileInfo
}

func min(a, b int64) int64 {
	if a > b {
		return b
	}
	return a
}

func isReasonable(name string) (bool, int64) {
	// Immediately catch Stdin.
	if name == "-" {
		return false, 0
	}

	info, err := os.Stat(name)
	if err != nil {
		return false, 0
	}

	return info.Mode().IsRegular() &&
		info.Size() <= min(10*1024*1024, sysinfo.PhysmemAvailable()/2), info.Size()
}

func getFileList(name string, size int64) ([]string, int) {
	fi, err := os.Open(name)
	if err != nil {
		fatal("%s", err.Error())
	}
	defer fi.Close()

	buf := make([]byte, size)

	_, err = fi.Read(buf)
	if err != nil && err != io.EOF {
		fatal("%s", err.Error())
	}

	var list []string

	count := 0
	for i := 0; i < len(buf); i++ {
		if buf[i] != nullByte {
			o := bytes.IndexByte(buf[i:], nullByte)
			if o < 0 {
				break
			}
			list = append(list, string(buf[i:i+o]))
			i += o
		}
		count++
	}
	return list, count
}

func getFileStatus(names []string) []fstatus {
	n := len(names)
	if n == 0 || (n == 1 && printOne) {
		return []fstatus{fstatus{failed: errNoStat}}
	}

	nf := 1
	if n > 1 {
		nf = n
	}

	f := make([]fstatus, nf)
	for i, name := range names {
		var (
			info os.FileInfo
			err  error
		)
		if name == "" || name == "-" {
			info, err = os.Stdin.Stat()
		} else {
			info, err = os.Stat(name)
		}
		f[i] = fstatus{failed: err, stat: info}
	}
	return f
}

func findNumberWidth(f []fstatus) int {
	width := 1

	if 0 < len(f) && f[0].failed != errNoStat {
		minWidth := 1
		reg := int64(0)

		for _, fs := range f {
			if fs.failed == nil {
				if fs.stat.Mode().IsRegular() {
					reg += fs.stat.Size()
				} else {
					minWidth = 7
				}
			}
		}

		for ; 10 <= reg; reg /= 10 {
			width++
		}

		if width < minWidth {
			width = minWidth
		}
	}
	return width
}

func writeCounts(lines, words, chars, numBytes, lineLength int64, fname string) {

	const fmtSpInt = " %*d"
	fmtInt := "%*d"

	if *printLines {
		fmt.Printf(fmtInt, numberWidth, lines)
		fmtInt = fmtSpInt
	}
	if *printWords {
		fmt.Printf(fmtInt, numberWidth, words)
		fmtInt = fmtSpInt
	}
	if *printChars {
		fmt.Printf(fmtInt, numberWidth, chars)
		fmtInt = fmtSpInt
	}
	if *printBytes {
		fmt.Printf(fmtInt, numberWidth, numBytes)
		fmtInt = fmtSpInt
	}
	if *printLineLength {
		fmt.Printf(fmtInt, numberWidth, lineLength)
	}
	fmt.Printf(" %s\n", fname)
}

func wcFile(name string, status fstatus) int {
	if name == "" || name == "-" {
		return wc(os.Stdin, -1, status)
	}
	fi, err := os.Open(name)
	if _, ok := err.(*os.PathError); ok {
		print("%s", err.Error())
		return 1
	}

	ok := wc(fi, 0, status)
	if err := fi.Close(); err != nil {
		print("%v", err)
		return 1
	}
	return ok
}

func main() {
	flag.Usage = func() {
		fmt.Printf(`Usage: %s [OPTION]... [FILE]...
  or:  wc [OPTION]... --files0-from=F
Print newline, word, and byte counts for each FILE, and a total line if
more than one FILE is specified.  A word is a non-zero-length sequence of
characters delimited by white space.

With no FILE, or when FILE is -, read standard input.

The options below may be used to select which counts are printed, always in
the following order: newline, word, character, byte, maximum line length.
`, flag.Program)
		flag.PrintDefaults()
		fmt.Printf(`
Report %s bugs to ericscottlagergren@gmail.com
Go coreutils home page: <https://www.github.com/EricLagergren/go-coreutils/>
`, flag.Program)
		os.Exit(0)
	}
	flag.ProgVersion = "2.2"
	flag.Parse()

	if *constVersion {
		fmt.Printf("Unicode Version: %s\n", unicode.Version)
		os.Exit(0)
	}

	if !(*printBytes || *printChars ||
		*printLines || *printWords || *printLineLength) {
		*printLines = true
		*printBytes = true
		*printWords = true
	}

	// This is a gross attempt to simulate this...
	// (print_lines + print_words + print_chars +
	//	print_bytes + print_linelength) == 1
	//
	// Since Go can't add booleans (e.g. false + true == 1)
	// and checking that *only* one of 5 bool variables would be sloppy,
	// we check the number of set flags and the remaining non-'print' flags
	// which is a much smaller set of conditions to check
	//
	//	I could just use a loop, but this also removes a branch soooo...
	printOne =
		// 1 flag and it's not *filesFrom OR *tabWidth
		((flag.NFlag() == 1 && *filesFrom == "" && *tabWidth == 8) ||
			// 2 flags and one is *filesFrom OR *tabWidth
			(flag.NFlag() == 2 && (*filesFrom != "" || *tabWidth != 8)) ||
			// 3 flags and two are *filesFrom AND *tabWidth
			(flag.NFlag() == 3 && *filesFrom != "" && *tabWidth != 8))

	var (
		ok         int           // Return status.
		files      = flag.Args() // List of files.
		numFiles   = len(files)  // Number of files to wc.
		reasonable = true        // Can we read file list into memory?
		size       int64
	)

	if *filesFrom != "" {
		// Cannot specify files with --files0-from.
		if flag.NArg() > 0 {
			fatal("file operands cannot be combined with --files0-from")
		}

		// --files0-from is small enough to fit into RAM.
		if reasonable, size = isReasonable(*filesFrom); reasonable {
			files, numFiles = getFileList(*filesFrom, size)
		}
	}

	fs := getFileStatus(files)
	numberWidth = findNumberWidth(fs)

	if reasonable {
		if files == nil || len(files) == 0 {
			files = []string{"-"}
		}
		for i, file := range files {
			ok ^= wcFile(file, fs[i])
		}
	} else {
		var err error

		file := os.Stdin
		if *filesFrom != "-" {
			file, err = os.Open(*filesFrom)
		}

		if err != nil {
			fatal("cannot open %q for reading: No such file or directory", *filesFrom)
		}
		defer file.Close()

		s := bufio.NewScanner(file)
		s.Split(func(data []byte, atEOF bool) (advance int, token []byte, err error) {
			if atEOF && len(data) == 0 {
				return 0, nil, nil
			}
			if i := bytes.IndexByte(data, nullByte); i >= 0 {
				// We have a full newline-terminated line.
				return i + 1, dropnullByte(numFiles, data[0:i]), nil
			}
			// If we're at EOF, we have a final, non-terminated line. Return it.
			if atEOF {
				return len(data), dropnullByte(numFiles, data), nil
			}
			// Request more data.
			return 0, nil, nil
		})
		for ; s.Scan(); numFiles++ {
			ok ^= wcFile(s.Text(), fstatus{})
		}
		if err := s.Err(); err != nil {
			fatal("%v", err)
		}
	}

	if numFiles > 1 {
		writeCounts(totalLines, totalWords,
			totalChars, totalBytes, maxLineLength, "total")
	}

	// Return status.
	os.Exit(ok)
}

func dropnullByte(i int, data []byte) []byte {
	if len(data) > 0 && data[len(data)-1] == nullByte {
		return data[0 : len(data)-1]
	}
	fatal("invalid zero-length file name at position: %d", i)
	panic("unreachable")
}
