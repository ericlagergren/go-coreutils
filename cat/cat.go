// Copyright (c) 2014-2016 Eric Lagergren
// Use of this source code is governed by the GPL v3 or later.

package main

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"os"

	"github.com/EricLagergren/go-coreutils/internal/flag"
)

var (
	all        = flag.BoolP("show-all", "A", false, "equivalent to -vET")
	blank      = flag.BoolP("number-nonblank", "b", false, "number nonempty output lines, overrides -n")
	npEnds     = flag.BoolP("ends", "e", false, "equivalent to -vE")
	ends       = flag.BoolP("show-ends", "E", false, "display $ at end of each line")
	number     = flag.BoolP("number", "n", false, "number all output lines")
	squeeze    = flag.BoolP("squeeze-blank", "s", false, "suppress repeated empty output lines")
	npTabs     = flag.BoolP("tabs", "t", false, "equivalent to -vT")
	tabs       = flag.BoolP("show-tabs", "T", false, "display TAB characters as ^I")
	nonPrint   = flag.BoolP("non-printing", "v", false, "use ^ and M- notation, except for LFD and TAB")
	unbuffered = flag.BoolP("unbuffered", "u", false, "(ignored)")

	totalNewline    int64
	showNonPrinting bool
	simple          bool

	fatal = log.New(os.Stderr, "", 0)
)

const caret = '^'

var (
	emdash   = []byte("M-")
	horizTab = []byte("^I")
	delete_  = []byte("^?")
)

const (
	lineLen = 20
	lineEnd = lineLen - 2
)

var (
	lineBuf = [...]byte{
		' ', ' ', ' ', ' ', ' ',
		' ', ' ', ' ', ' ', ' ',
		' ', ' ', ' ', ' ', ' ',
		' ', ' ', ' ', '0', '\t',
	}
	linePrint = lineLen - 7
	lineStart = lineLen - 2
)

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func nextLineNum() {
	ep := lineEnd
	for {
		// if it's possible, increment the line number
		if lineBuf[ep] < '9' {
			lineBuf[ep]++
			return
		}

		// otherwise, set it to 0 and move backwards
		lineBuf[ep] = '0'
		ep--

		// stop when we've moved past our printing area
		if ep < lineStart {
			break
		}
	}

	// who needs pointer arithmetic? ...said nobody ever
	if lineStart < len(lineBuf) {
		lineStart--
		lineBuf[lineStart] = '1'
	} else {
		lineBuf[0] = '>'
	}

	if lineStart < linePrint {
		linePrint--
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
	var eob int              // end of buffer
	bpin := eob + 1          // beginning of buffer
	var ch byte              // char in buffer
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
						w.Write(lineBuf[linePrint:])
					}
				}

				// Add '$' at EOL if requested
				if *ends {
					w.WriteByte('$')
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
			w.Write(lineBuf[linePrint:])
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
						w.Write(delete_)
					} else {
						w.Write(emdash)
						if ch >= 128+32 {
							if ch < 128+127 {
								w.WriteByte(ch - 128)
							} else {
								w.Write(delete_)
							}
						} else {
							w.WriteByte(caret)
							w.WriteByte(ch - 128 + 64)
						}
					}
				} else if ch == 9 && !*tabs {
					w.WriteByte(9)
				} else if ch == 10 {
					newlines = -1
					break
				} else {
					w.WriteByte(caret)
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
					w.Write(horizTab)
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

func init() {
	flag.Usage = func() {
		fmt.Printf(`Usage: %s [OPTION]... [FILE]...
Concatenate FILE(s), or standard input, to standard output.

`, flag.Program)
		flag.DBE()
	}
	flag.ProgVersion = "2.0"
	flag.Parse()

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
	if !(*number || *ends || showNonPrinting ||
		*tabs || *squeeze) {
		simple = true
	}
}
