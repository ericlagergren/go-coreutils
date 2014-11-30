/*
	Go wc - print the lines, words, bytes, and characters in files
	Copyright (C) 2014 Eric Lagergren

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

// +build windows

package main

import (
	"bufio"
	"bytes"
	"fmt"
	flag "github.com/ogier/pflag"
	"io"
	"os"
	"text/tabwriter"
	"unicode"
	"unicode/utf8"
)

// VERSION and HELP output inspired by GNU coreutils
const (
	VERSION = `Go wc (Go coreutils) 1.0
Copyright (C) 2014 Eric Lagergren
License GPLv3+: GNU GPL version 3 or later <http://gnu.org/licenses/gpl.html>.
This is free software: you are free to change and redistribute it.
There is NO WARRANTY, to the extent permitted by law.

Written by Eric Lagergren <ericscottlagergren@gmail.com>
Inspired by Written by Paul Rubin and David MacKenzie.`
	HELP = `Usage: wc [OPTION]... [FILE]
   or: wc [OPTION]... --files-from [FILE LIST]
Print newline, word, and byte counts for each FILE, and a total line if
more than one FILE is specified.  With no FILE, or when FILE is -,
read standard input.  A word is a non-zero-length sequence of characters
delimited by white space.
The options below may be used to select which counts are printed, always in
the following order: newline, word, character, byte, maximum line length.
  -c, --bytes            print the byte counts
  -m, --chars            print the character counts
  -l, --lines            print the newline counts
  -f, --files0-from=F    read input from the files specified by
                           NUL-terminated names in file F;
                           If F is - then read names from standard input
  -L, --max-line-length  print the length of the longest line
  -w, --words            print the word counts
      --help     display this help and exit
      --version  output version information and exit

Report wc bugs to ericscottlagergren@gmail.com
Go coreutils home page: <https://www.github.com/EricLagerg/go-coreutils/>
`
	NEW_LINE = rune('\n')
	RETURN   = rune('\r')
	F_FEED   = rune('\f')
	H_TAB    = rune('\t')
	V_TAB    = rune('\v')
	SPACE    = rune(' ')
	NULL     = rune(0x0)
)

var (
	inFile *os.File
	err    error

	// We can't declare these as constants
	ONE_BYTE      = []uint8{}
	NEW_LINE_BYTE = []uint8{'\n'}
	BUFFER        = make([]uint8, 64*1024)

	// Our cumulative number of lines, words, chars, and bytes
	totalLines    int64
	totalWords    int64
	totalChars    int64
	totalBytes    int64
	maxLineLength int64

	// Our cli args
	printLines      = flag.BoolP("lines", "l", false, "print the newline counts\n")
	printWords      = flag.BoolP("words", "w", false, "print the word counts\n")
	printChars      = flag.BoolP("chars", "m", false, "print the character counts\n")
	printBytes      = flag.BoolP("bytes", "c", false, "print the byte counts\n")
	printLineLength = flag.BoolP("max-line-length", "L", false, "print the length of the longest line\n")
	filesFrom       = flag.StringP("files-from0", "f", "", "read file names from file\n")
	constVersion    = flag.BoolP("unicode-version", "u", false, "print the \"Unicode edition\"\n")
	version         = flag.BoolP("version", "v", false, "print program's version\n")

	// Pretty print output
	tabWriter = tabwriter.NewWriter(os.Stdout, 3, 0, 2, ' ', tabwriter.AlignRight)
)

// Count counts the number of non-overlapping instances of sep in s.
// Borrowed and modified from http://golang.org/src/pkg/bytes/bytes.go?m=text
// Originally under BSD license (like rest of Go code)
func Count(s, sep []byte) int64 {
	n := len(sep)
	count := int64(0)
	c := sep[0]
	i := 0
	t := s[:len(s)-n+1]
	for i < len(t) {
		if t[i] != c {
			o := bytes.IndexByte(t[i:], c)
			if o < 0 {
				break
			}
			i += o
		}
		if n == 1 || bytes.Equal(s[i:i+n], sep) {
			count++
			i += n
			continue
		}
		i++
	}
	return count
}

func WC(fname string, stdin bool, ctr int) {
	// Our temp number of lines, words, chars, and bytes
	var (
		lines      int64
		words      int64
		chars      int64
		bytez      int64
		lineLength int64
		linePos    int64
		inWord     int64
		prev       = NULL
	)

	if stdin {
		inFile = os.Stdin
	} else {
		inFile, err = os.Open(fname)
		defer inFile.Close()

		if err != nil && err == err.(*os.PathError) {
			fmt.Printf("invalid filname \"%s\" (arg/line %v)\n", fname, ctr)
			return
		}
	}

	countComplicated := *printWords || *printLineLength

	// If we simply want the bytes we can ignore the overhead (see: GNU
	// wc.c by Paul Rubin and David MacKenzie) of counting lines, chars,
	// and words
	if *printBytes && !*printChars && !*printLines && !countComplicated {

		// A syscall is quicker than reading each byte of the file
		statFile, err := inFile.Stat()

		if err == nil {
			bytez = statFile.Size()
		} else {
			panic(err)
		}

		// Manually count bytes if Stat() fails or if we're reading from
		// piped input (e.g. cat file.csv | wc -c -)
		if err != nil || statFile.Mode()&os.ModeNamedPipe != 0 {

			for {
				inBuffer, err := inFile.Read(BUFFER)

				if err != nil && err != io.EOF {
					panic(err)
				}

				bytez += int64(inBuffer)

				if err == io.EOF {
					break
				}
			}
		}

		// Use a different loop to lower overhead if we're *only* counting
		// lines (or lines and bytes)
	} else if !*printChars && !countComplicated {
		for {

			// Technically Stat() is the fastest way to return file size,
			// but since we have to read the file to get the number of lines
			// we might as well just return the amount of bytes from calling
			// Read()

			inBuffer, err := inFile.Read(BUFFER)

			if err != nil && err != io.EOF {
				panic(err)
			}

			lines += Count(BUFFER[:inBuffer], NEW_LINE_BYTE)
			bytez += int64(inBuffer)

			if err == io.EOF {
				break
			}
		}
	} else {
		for {
			inBuffer, err := inFile.Read(BUFFER)
			b := BUFFER[:inBuffer]

			for len(b) > 0 {
				r, s := utf8.DecodeRune(b)
				inWord := int64(1)

				switch r {
				case NEW_LINE:
					lines++
					if linePos > lineLength {
						lineLength = linePos
					}
					linePos = 0
					if prev == NEW_LINE {
						linePos++
						words += inWord
						inWord = 1
					}
				case RETURN:
					fallthrough
				case F_FEED:
					if linePos > lineLength {
						lineLength = linePos
					}
					linePos = 0
					words += inWord
					inWord = 1
				case H_TAB:
					if prev == H_TAB {
						// Don't double count tabs.
						// For instance, in a tab-delimited CSV file:
						//
						// FIRST_NAME, MIDDLE_NAME, LAST_NAME
						// JOHN\t\tDOE
						//
						// Without the break, we'd count the \t\t as
						// an additional word
						linePos += 8 - (linePos % 8)
						break
					}
					linePos += 8 - (linePos % 8)
					words += inWord
					inWord = 1
				case SPACE:
					linePos++
				case V_TAB:
					fallthrough
				default:
					switch prev {
					case NULL:
						linePos++
						words += inWord
						inWord = 1
					case H_TAB:
						linePos++
					}

					if unicode.IsPrint(prev) {
						linePos++
						if unicode.IsSpace(prev) {
							words += inWord
							inWord = 1
						}
						inWord = 0
					}
					break
				}
				chars++
				bytez += int64(s)
				b = b[s:]
				prev = r
			}

			if err != nil && err != io.EOF {
				panic(err)
			}

			if err == io.EOF {
				break
			}

			if linePos > lineLength {
				lineLength = linePos
			}
			words += inWord
			// Catch case of file ending in a new line where we add an
			// additional word
			if prev == NEW_LINE {
				words--
			}
		}
	}

	totalBytes += bytez
	totalChars += chars
	totalLines += lines
	totalWords += words

	if lineLength > maxLineLength {
		maxLineLength = lineLength
	}

	if *printLines {
		fmt.Fprintf(tabWriter, "%d\t", lines)
	}
	if *printWords {
		fmt.Fprintf(tabWriter, "%d\t", words)
	}
	if *printChars {
		fmt.Fprintf(tabWriter, "%d\t", chars)
	}
	if *printBytes {
		fmt.Fprintf(tabWriter, "%d\t", bytez)
	}
	if *printLineLength {
		fmt.Fprintf(tabWriter, "%d\t", lineLength)
	}
	fmt.Fprintf(tabWriter, " %s\n", fname)
}

func main() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "%s", HELP)
		os.Exit(0)
	}
	flag.Parse()
	args := flag.Args()

	if *constVersion {
		fmt.Fprintf(tabWriter, "Unicode Version: %s\n", unicode.Version)
		tabWriter.Flush()
		os.Exit(0)
	} else if *version {
		fmt.Fprintf(tabWriter, "%s\n", VERSION)
		tabWriter.Flush()
		os.Exit(0)
	}

	if len(args) > 0 && args[0] != "-" {
		for i, f := range args {
			WC(f, false, i)
		}
	} else if *filesFrom != "" {
		i := 0
		file, err := os.Open(*filesFrom)
		if err != nil {
			panic(err)
		}
		defer file.Close()
		r := bufio.NewReader(file)
		for {
			l, _, err := r.ReadLine()
			WC(string(l), false, i)
			if err == io.EOF {
				break
			}
			i++
		}
	} else {
		WC("-", true, 0)
	}

	defer tabWriter.Flush()

	if len(args) > 1 {
		if *printLines {
			fmt.Fprintf(tabWriter, "%d\t", totalLines)
		}
		if *printWords {
			fmt.Fprintf(tabWriter, "%d\t", totalWords)
		}
		if *printChars {
			fmt.Fprintf(tabWriter, "%d\t", totalChars)
		}
		if *printBytes {
			fmt.Fprintf(tabWriter, "%d\t", totalBytes)
		}
		if *printLineLength {
			fmt.Fprintf(tabWriter, "%d\t", maxLineLength)
		}
		fmt.Fprintf(tabWriter, " total\n")
	}
}
