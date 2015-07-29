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

/*
	Written by Eric Lagergren <ericscottlagergren@gmail.com>
	Inspired by GNU's wc, which was written by
	Paul Rubin, phr@ocf.berkeley.edu and David MacKenzie, djm@gnu.ai.mit.edu
*/

package wc

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"log"
	"os"

	"github.com/EricLagerg/go-gnulib/sysinfo"
	"github.com/EricLagerg/go-gnulib/ttyname"

	flag "github.com/ogier/pflag"
)

const (
	Version = `Go wc (Go coreutils) 2.1
Copyright (C) 2015 Eric Lagergren
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
	Null        = rune(0x0)
	NewLineByte = 10
	NullByte    = 0
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
	errNoStat = errors.New("No stat.")

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

	// fatal.Fatal helper
	//fatal = log.New(os.Stderr, "", log.Lshortfile)
	fatal = log.New(os.Stderr, "", 0)
)

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

func min(a, b uint64) uint64 {
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

	if info.Mode().IsRegular() &&
		uint64(info.Size()) <= min(10*1024*1024, sysinfo.PhysmemAvail()/2) {
		return true, info.Size()
	}

	return false, info.Size()
}

func getFileList(name string, size int64) ([]string, int) {

	fi, err := os.Open(name)
	if err != nil {
		return nil, -1
	}
	defer fi.Close()

	// buffer to hold file
	buf := make([]byte, size)

	_, err = fi.Read(buf)
	if err != nil && err != io.EOF {
		return nil, -1
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

func getFileStatus(n int, names []string) []*fstatus {
	nf := 1
	if n > 1 {
		nf = n
	}

	f := make([]*fstatus, nf)

	if n == 0 || (n == 1 && printOne) {
		f[0] = &fstatus{errNoStat, nil}
	} else {
		for i := 0; i < n; i++ {
			var (
				info os.FileInfo
				err  error
			)
			if names == nil || names[i] == "-" || names[i] == "" {
				info, err = os.Stdin.Stat()
			} else {
				info, err = os.Stat(names[i])
			}
			f[i] = &fstatus{err, info}
		}
	}

	return f
}

func findNumberWidth(n int, f []*fstatus) int {
	width := 1

	if 0 < n && f[0].failed != errNoStat {
		minWidth := 1
		reg := int64(0)

		for i := 0; i < n; i++ {
			if f[i].failed == nil {
				if f[i].stat.Mode().IsRegular() {
					reg += f[i].stat.Size()
				} else {
					minWidth = 7
				}
			}
		}

		for ; 10 < reg; reg /= 10 {
			width++
		}

		if width < minWidth {
			width = minWidth
		}
	}

	return width
}

func writeCounts(lines, words, chars, numBytes, lineLength int64, fname string) {

	const fmtIntSp = " %*d"
	fmtInt := "%*d"

	if *printLines {
		fmt.Printf(fmtInt, numberWidth, lines)
		fmtInt = fmtIntSp
	}
	if *printWords {
		fmt.Printf(fmtInt, numberWidth, words)
		fmtInt = fmtIntSp
	}
	if *printChars {
		fmt.Printf(fmtInt, numberWidth, chars)
		fmtInt = fmtIntSp
	}
	if *printBytes {
		fmt.Printf(fmtInt, numberWidth, numBytes)
		fmtInt = fmtIntSp
	}
	if *printLineLength {
		fmt.Printf(fmtInt, numberWidth, lineLength)
		fmtInt = fmtIntSp
	}
	fmt.Printf(" %s\n", fname)
}

func wcFile(name string, status *fstatus) int {
	if name == "" || name == "-" {
		if !ttyname.IsAtty(os.Stdin.Fd()) {
			return wc(os.Stdin, -1, status)
		}
	} else {
		fi, err := os.Open(name)
		if err != nil && err == err.(*os.PathError) {
			fatal.Printf("%s No such file or directory\n", name)
			return 1
		}

		ok := wc(fi, 0, status)
		if err := fi.Close(); err != nil {
			return 1
		}
		return ok
	}
	// Unreachable.
	return 1
}

func DO() error {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "%s", Help)
		os.Exit(1)
	}
	flag.Parse()

	// if *constVersion {
	// 	fmt.Printf("Unicode Version: %s\n", unicode.Version)
	// 	os.Exit(0)
	// } else if *version {
	// 	fmt.Printf("%s\n", Version)
	// 	os.Exit(0)
	// }

	if !(*printBytes || *printChars || *printLines || *printWords || *printLineLength) {
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
	// 1 flag and it's --files0-from
	if (flag.NFlag() == 1 && *filesFrom == "" && *tabWidth == 8) ||
		// 2 flags and one's *filesFrom OR *tabWidth
		(flag.NFlag() == 2 && (*filesFrom != "" || *tabWidth != 8)) ||
		// 3 flags and two are *filesFrom AND *tabWidth
		(flag.NFlag() == 3 && *filesFrom != "" && *tabWidth != 8) {

		printOne = true
	}

	var (
		ok         = 0           // Return status.
		files      = flag.Args() // List of files.
		numFiles   = len(files)  // Number of files to wc.
		reasonable = true        // Can we read file list into memory?
		stdin      = true        // Are we reading from stdin?
		size       int64
	)

	if *filesFrom != "" {
		// Cannot specify files with --files0-from.
		if flag.NArg() > 0 {
			return fmt.Errorf("file operands cannot be combined with --files0-from")
		}

		// --files0-from is small enough to fit into RAM.
		if reasonable, size = isReasonable(*filesFrom); reasonable {
			files, numFiles = getFileList(*filesFrom, size)
			stdin = false
		}
	}

	fs := getFileStatus(numFiles, files)
	numberWidth = findNumberWidth(numFiles, fs)

	if !reasonable {
		var (
			file *os.File
			err  error
		)

		file = os.Stdin
		if *filesFrom != "-" {
			file, err = os.Open(*filesFrom)
		}

		if err != nil {
			return fmt.Errorf("cannot open: %s\n", *filesFrom)
		}
		defer file.Close()

		r := bufio.NewReader(file)
		i := 0
		for {
			fname, err := r.ReadString(NullByte)
			if err != nil {
				if err != io.EOF && i < 1 {
					return fmt.Errorf(err.Error())
				}
				break
			}

			if len(fname) > 1 {
				// Trim trailing null-byte.
				if fname[len(fname)-1] == NullByte {
					fname = fname[:len(fname)-1]
				}
			} else {
				return fmt.Errorf("invalid zero-length file name at position: %d\n", i)
			}

			ok ^= wcFile(fname, nil)
			i++
		}

		numFiles = i
	} else {
		if stdin { // We're reading from Stdin.
			ok ^= wcFile("", fs[0])
		} else {
			for i, v := range files {
				ok ^= wcFile(v, fs[i])
			}
		}
	}

	if numFiles > 1 {
		writeCounts(totalLines, totalWords,
			totalChars, totalBytes, maxLineLength, "total")
	}

	// Return status.
	// os.Exit(ok)
	if ok == 0 {
		return nil
	}
	return fmt.Errorf("exit is non-zero")
}

func main() {}
