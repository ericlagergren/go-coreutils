/*
	Go wc - print the lines, words, bytes, and characters in files

	Copyright (C) 2015 Eric Lagergren

	This program is free software: you can redistribute it and/or modify
	it under the terms of the GNU General Public License as published by
	the Free Software Foundation, either version 3 of the License, or
	(at your option) any later version.

	This program is distributed in the hope that it will be useful,
	but WITHOUT ANY WARRANTY; without even the implied warranty of
	MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
	GNU General Public License for more details.

	You should have received a copy of the GNU General Public License
	along with this program.  If not, see <http://www.gnu.org/licenses/>.
*/

// Written by Eric Lagergren <ericscottlagergren@gmail.com>

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

	"github.com/EricLagerg/go-gnulib/sysinfo"

	flag "github.com/ogier/pflag"
)

const (
	Version = `Go wc (Go coreutils) 2.2
Copyright (C) 2014-2015 Eric Lagergren
License GPLv3+: GNU GPL version 3 or later <http://gnu.org/licenses/gpl.html>.
This is free software: you are free to change and redistribute it.
There is NO WARRANTY, to the extent permitted by law.
`

	Help = `Usage: wc [OPTION]... [FILE]
   or: wc [OPTION]... --files0-from [FILE LIST]

Print newline, word, and byte counts for each FILE, and a total line if
more than one FILE is specified.  With no FILE, or when FILE is -,
read standard input.  A word is a non-zero-length sequence of characters
delimited by white space.
The options below may be used to select which counts are printed, always in
the following order: newline, word, character, byte, maximum line length.
  -c, --bytes            print the byte counts
  -m, --chars            print the character counts
  -l, --lines            print the newline counts
      --files0-from=F    read input from NUL-terminated string inside F
                         * If F is - then read names from standard input
  -L, --max-line-length  print the length of the longest line
  -w, --words            print the word counts
  -t, --tab              change tab width
  -h, --help             display this help and exit
  -u, --unicode-version  display unicode version used and exit
  -v, --version  output  version information and exit

Report wc bugs to ericscottlagergren@gmail.com
Go coreutils home page: <https://www.github.com/EricLagerg/go-coreutils/>
`
	NewLine     = rune('\n')
	Return      = rune('\r')
	FormFeed    = rune('\f')
	HorizTab    = rune('\t')
	VertTab     = rune('\v')
	Space       = rune(' ')
	NewLineByte = 10
	NullByte    = 0x00
	BufferSize  = (64 * 1024)
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
	printLines      = flag.BoolP("lines", "l", false, "")
	printWords      = flag.BoolP("words", "w", false, "")
	printChars      = flag.BoolP("chars", "m", false, "")
	printBytes      = flag.BoolP("bytes", "c", false, "")
	printLineLength = flag.BoolP("max-line-length", "L", false, "")
	filesFrom       = flag.String("files0-from", "", "")
	tabWidth        = flag.Int64P("tab", "t", 8, "")
	constVersion    = flag.BoolP("unicode-version", "u", false, "")
	version         = flag.BoolP("version", "v", false, "")

	logger = log.New(os.Stderr, "", 0)

	programName = binName()
)

func binName() string {
	file := os.Args[0]
	var i int
	for i = len(file) - 1; i > 0 && file[i] != '/'; i-- {
	}
	return file[i+1:]
}

func fatal(format string, v ...interface{}) {
	logger.Fatalf("%s: %s\n", programName, fmt.Sprintf(format, v...))
}

func print(format string, v ...interface{}) {
	logger.Printf("%s: %s\n", programName, fmt.Sprintf(format, v...))
}

type fstatus struct {
	failed error
	stat   os.FileInfo
}

func count(s []byte, delim byte) int64 {
	count := int64(0)
	i := 0
	for i < len(s) {
		if s[i] != delim {
			o := bytes.IndexByte(s[i:], delim)
			if o < 0 {
				break
			}
			i += o
		}
		count++
		i++
	}
	return count
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
		return false, -1
	}

	info, err := os.Stat(name)
	if err != nil {
		return false, -1
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
	i := 0
	for i < len(buf) {
		if buf[i] != NullByte {
			o := bytes.IndexByte(buf[i:], NullByte)
			if o < 0 {
				break
			}
			list = append(list, string(buf[i:i+o]))
			i += o
		}
		count++
		i++
	}
	return list, count
}

func getFileStatus(names []string) []fstatus {
	nf := 1
	n := len(names)
	if n > 1 {
		nf = n
	}

	f := make([]fstatus, nf)

	if n == 0 || (n == 1 && printOne) {
		f[0] = fstatus{errNoStat, nil}
	} else {
		for i, name := range names {
			var (
				info os.FileInfo
				err  error
			)
			if name == "-" || name == "" {
				info, err = os.Stdin.Stat()
			} else {
				info, err = os.Stat(name)
			}
			f[i] = fstatus{err, info}
		}
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
		fmt.Fprintf(os.Stderr, "%s", Help)
		os.Exit(0)
	}
	flag.Parse()

	if *constVersion {
		fmt.Printf("Unicode Version: %s\n", unicode.Version)
		os.Exit(0)
	} else if *version {
		fmt.Printf("%s\n", Version)
		os.Exit(0)
	}

	*printLines = !(*printBytes || *printChars || *printLines || *printWords || *printLineLength)
	*printBytes = *printLines
	*printWords = *printBytes

	// This is a gross attempt to simulate this...
	// (print_lines + print_words + print_chars +
	//	print_bytes + print_linelength) == 1
	//
	// Since Go can't add booleans (e.g. false + true == 1)
	// and checking that *only* one of 5 bool variables would be sloppy,
	// we check the number of set flags and the remaining non-'print' flags
	// which is a much smaller set of conditions to check
	//
	// 1 flag and it's --files0-from
	printOne = ((flag.NFlag() == 1 && *filesFrom == "" && *tabWidth == 8) ||
		// 2 flags and one's *filesFrom OR *tabWidth
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

		i := 0
		for r := bufio.NewReader(file); ; i++ {
			fname, err := r.ReadString(NullByte)
			if err != nil {
				if err != io.EOF && i < 1 {
					fatal("%v", err)
				}
				break
			}

			// Trim trailing null-byte.
			if len(fname) > 1 && fname[len(fname)-1] == NullByte {
				fname = fname[:len(fname)-1]
			} else {
				fatal("invalid zero-length file name at position: %d", i)
			}

			ok ^= wcFile(fname, fstatus{})
		}
		numFiles = i
	}

	if numFiles > 1 {
		writeCounts(totalLines, totalWords,
			totalChars, totalBytes, maxLineLength, "total")
	}

	// Return status.
	os.Exit(ok)
}
