/*
	Go cat - concatenate files and print on the standard output.
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
*/

package main

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"os"
	"syscall"

	k32 "github.com/EricLagerg/go-gnulib/windows"
	flag "github.com/ogier/pflag"
)

const (
	Help = `Usage: cat [OPTION]... [FILE]...
Concatenate FILE(s), or standard input, to standard output.

  -A, --show-all           equivalent to -vET
  -b, --number-nonblank    number nonempty output lines, overrides -n
  -e                       equivalent to -vE
  -E, --show-ends          display $ at end of each line
  -n, --number             number all output lines
  -s, --squeeze-blank      suppress repeated empty output lines
  -t                       equivalent to -vT
  -T, --show-tabs          display TAB characters as ^I
  -u                       (ignored)
  -v, --show-nonprinting   use ^ and M- notation, except for LFD and TAB
      --Help     display this Help and exit
      --version  output version information and exit

With no FILE, or when FILE is -, read standard input.

Examples:
  cat        Copy standard input to standard output.

Report wc bugs to ericscottlagergren@gmail.com
Go coreutils home page: <https://www.github.com/EricLagerg/go-coreutils/>
`
	Version = `Go cat (Go coreutils) 2.0
Copyright (C) 2014 Eric Lagergren
License GPLv3+: GNU GPL version 3 or later <http://gnu.org/licenses/gpl.html>.
This is free software: you are free to change and redistribute it.
There is NO WARRANTY, to the extent permitted by law.

Written by Eric Lagergren <ericscottlagergren@gmail.com>
`
)

var (
	all        = flag.BoolP("show-all", "A", false, "")
	blank      = flag.BoolP("number-nonblank", "b", false, "")
	npEnds     = flag.BoolP("ends", "e", false, "")
	ends       = flag.BoolP("show-ends", "E", false, "")
	number     = flag.BoolP("number", "n", false, "")
	squeeze    = flag.BoolP("squeeze-blank", "s", false, "")
	npTabs     = flag.BoolP("tabs", "t", false, "")
	tabs       = flag.BoolP("show-tabs", "T", false, "")
	nonPrint   = flag.BoolP("non-printing", "v", false, "")
	unbuffered = flag.BoolP("unbuffered", "u", false, "")
	version    = flag.Bool("version", false, "")

	totalNewline    int64
	showNonPrinting bool

	// fatal = log.New(os.Stderr, "", 0)
	fatal = log.New(os.Stderr, "", log.Lshortfile)
)

const Caret = '^'

var (
	MDash    = []byte("M-")
	HorizTab = []byte("^I")
	Delete   = []byte("^?")

	LineTerm = []byte("$")

	LineLen = 20
	LineBuf = []byte{
		' ', ' ', ' ', ' ', ' ',
		' ', ' ', ' ', ' ', ' ',
		' ', ' ', ' ', ' ', ' ',
		' ', ' ', ' ', '0', '\t',
	}
	LinePrint = LineLen - 7
	LineStart = LineLen - 2
	LineEnd   = LineLen - 2
)

func nextLineNum() {
	ep := LineEnd
	for {
		// if it's possible, increment the line number
		if LineBuf[ep] < '9' {
			LineBuf[ep]++
			return
		}

		// otherwise, set it to 0 and move backwards
		LineBuf[ep] = '0'
		ep--

		// stop when we've moved past our printing area
		if ep < LineStart {
			break
		}
	}

	// who needs pointer arithmetic? ...said nobody ever
	if LineStart < len(LineBuf) {
		LineStart--
		LineBuf[LineStart] = '1'
	} else {
		LineBuf[0] = '>'
	}

	if LineStart < LinePrint {
		LinePrint--
	}
}

// simple cat, meaning no formatting -- just copy from input to stdout
func simpleCat(r io.Reader, w io.Writer) int {
	_, err := io.Copy(w, r)
	if err != nil {
		fatal.Fatalln(err)
	}
	return 0 // success! :-)
}

func cat(r io.Reader, buf []byte, w *bufio.Writer) int {
	newlines := totalNewline // total newlines across invocations
	eob := 0                 // end of buffer
	bpin := eob + 1          // beginning of buffer
	ch := byte(0)            // char in buffer
	size := len(buf) - 1     // len of buffer with room for sentinel byte

	// When I first tried translating this from C the algorithm
	// Torbjorn and rms used sort of confused me, so I'll try to explain
	// it as best I can.
	//
	// Because newlines indicate the end of a line of output, we deal
	// with that case first. In order to do this, we have a specific
	// part of our mess of for-loops that will continue to loop until
	// we run into a character that isn't a newline. When that happens,
	// we break out and start printing out all the other characters. (Well,
	// "printing" into our buffered output.)
	//
	// While printing, we break the loops if a newline character is found
	// so we can deal with it.

	// We don't break from this outermost loop until we return.
	for {
		// This is Go's version of a do-while loop. At the bottom of the
		// loop you'll find the condition `if ch != 10 { break }` which
		// is effectively the same as:
		// do
		//     {
		//			/* code */
		//     }
		// while(ch == '\n')
		for {
			// Initially we set bpin to 1, so this condition will always
			// be true our first time through. We do this so we can
			// immediately read into our buffer.
			//
			// Because we always increment bpin, (as you'll see later)
			// eventually it'll be larger than eob, (which marks the end
			// of our buffer). If that's the case, read() some more and
			// continue our loops.
			if bpin > eob {
				n, err := r.Read(buf[:size])
				if err == io.EOF {
					totalNewline = newlines
					w.Flush()
					return 0
				}
				if err != nil {
					totalNewline = newlines
					w.Flush()
					return 1
				}

				bpin = 0      // Reset bpin to the beginning of the buffer
				eob = n       // End of buffer is the number of bytes read
				buf[eob] = 10 // Place a sentinel at the end of the buffer
			} else {

				// If we don't have to read anything, we check to see if
				// We've seen more than 1 consecutive newline.
				// Yes, newlines == 0 means we've seen one newline.
				newlines++
				if newlines > 0 {
					if newlines >= 2 {
						newlines = 2

						// Multiple blank lines?
						if *squeeze {
							ch = buf[bpin]
							bpin++

							// Skip past all the printing parts of the loop
							continue
						}
					}

					// Line numbers for *empty* lines
					if *number && !*blank {
						nextLineNum()
						w.Write(LineBuf[LinePrint:])
					}
				}

				// Add '$' at EOL if requested
				if *ends {
					w.Write(LineTerm)
				}

				// Write the newline because we haven't printed it yet!
				w.WriteByte(10)
			}

			// If it's our first time in the loop ch is given a value for
			// the first time. Afterwards, we increment bpin. We do this
			// do simulate `ch = *bpin++`, which means `ch = BUFFER[bpin]`
			// and then increments bpin. (Pointer arithmetic.)
			ch = buf[bpin]
			bpin++

			// Here's the while portion of the simulated do-while loop.
			// Since while(true) is the same thing as if (!false), we
			// can do the same here -- while(ch == '\n') is the same
			// as `if ch != 10`. Note: '\n' == 10.
			if ch != 10 {
				break
			}
		}

		// Beginning of a line with line numbers requested?
		if newlines >= 0 && *number {
			nextLineNum()
			w.Write(LineBuf[LinePrint:])
		}

		// At this point ch will not be a newline, so we loop over
		// the entire buffer until we find a newline. If we find a newline,
		// we break back to the above loops. Generally, bpin will be less
		// than eob because our buffer is (usually) 4096 bytes, and
		// newlines (usually) occur more often than once per 4096 bytes.

		if showNonPrinting {
			for {
				if ch >= 32 {
					if ch < 127 {
						w.WriteByte(ch)
					} else if ch == 127 {
						w.Write(Delete)
					} else {
						w.Write(MDash)
						if ch >= 128+32 {
							if ch < 128+127 {
								w.WriteByte(ch - 128)
							} else {
								w.Write(Delete)
							}
						} else {
							w.WriteByte(Caret)
							w.WriteByte(ch - 128 + 64)
						}
					}
				} else if ch == 9 && !*tabs {
					w.WriteByte(9)
				} else if ch == 10 {
					newlines = -1
					break
				} else {
					w.WriteByte(Caret)
					w.WriteByte(ch + 64)
				}

				// Much like we did before, all we're doing is incrementing
				// our pointer (array index) after we give ch a new value.
				ch = buf[bpin]
				bpin++
			}
		} else {
			// Not non-printing
			for {
				if ch == 9 && *tabs {
					w.Write(HorizTab)
				} else if ch != 10 {
					w.WriteByte(ch)
				} else {
					newlines = -1
					break
				}

				// *bpin++
				ch = buf[bpin]
				bpin++
			}
		}
	}
}

func main() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "%s", Help)
		os.Exit(1)
	}
	flag.Parse()

	if *version {
		fmt.Fprintf(os.Stdout, "%s", Version)
		os.Exit(0)
	}

	var (
		ok     int  // return status
		simple bool // no non-printing
	)

	// -vET
	if *all {
		*nonPrint = true
		*npTabs = true
		*npEnds = true
	}
	if *npEnds {
		*ends = true
	}
	if *blank {
		*number = true
	}
	if *npTabs {
		*tabs = true
	}
	if *all || *npEnds || *npTabs || *nonPrint {
		showNonPrinting = true
	}

	outHandle := syscall.Handle(os.Stdout.Fd())
	outType, err := syscall.GetFileType(outHandle)
	if err != nil {
		fatal.Fatalln(err)
	}
	outBsize := 4096

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
		inHandle := syscall.Handle(file.Fd())
		inBsize := 4096

		// See http://stackoverflow.com/q/29360969/2967113
		// for why this differs from the Unix versions.
		//
		// Make sure we're not catting a file to itself,
		// provided it's a regular file. Catting a non-reg
		// file to itself is cool, e.g. cat file > file
		if outType == syscall.FILE_TYPE_DISK {

			inPath := make([]byte, syscall.MAX_PATH)
			outPath := make([]byte, syscall.MAX_PATH)

			err = k32.GetFinalPathNameByHandleA(inHandle, inPath, 0)
			if err != nil {
				fatal.Fatalln(err)
			}

			err = k32.GetFinalPathNameByHandleA(outHandle, outPath, 0)
			if err != nil {
				fatal.Fatalln(err)
			}

			if string(inPath) == string(outPath) {
				k, err := file.Seek(0, os.SEEK_CUR)
				if err != nil {
					panic(err)
				}
				fmt.Fprintf(os.Stderr, "%d", k)
				fmt.Fprintf(os.Stderr, "%d", inStat.Size())
				if n, _ := file.Seek(0, os.SEEK_CUR); n < inStat.Size() {
					fatal.Fatalf("%s: input file is output file\n", file.Name())
				}
			}
		}

		if simple {
			outBuf := bufio.NewWriterSize(os.Stdout, 4096)
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
