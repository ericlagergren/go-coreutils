package main

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"os"
	"syscall"

	"github.com/EricLagerg/go-gnulib/posix"
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

	//fatal = log.New(os.Stderr, "", 0)
	fatal = log.New(os.Stderr, "", log.Lshortfile)
)

var (
	Null      = []byte("^@")
	Eot       = []byte("^D")
	Bell      = []byte("^G")
	Backspace = []byte("^H")
	HorizTab  = []byte("^I")
	VertTab   = []byte("^K")
	FormFeed  = []byte("^L")
	Return    = []byte("^M")
	Escape    = []byte("^[")
	Delete    = []byte("^?")

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

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func nextLineNum() {
	ep := LineEnd
	for {
		// if it's possible, increment the line number
		if LineBuf[ep] < '9' {
			LineBuf[ep] += 1
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

// simple cat, meaning no formatting
func simpleCat(r io.Reader, w io.Writer) int {
	_, err := io.Copy(w, r)
	if err != nil {
		fatal.Fatalln(err)
	}
	return 0 // success!
}

func cat(r io.Reader, buf []byte, w *bufio.Writer) int {
	newlines := totalNewline
	ok := 0
	eob := 0
	bpin := eob + 1

	var ch byte
	for {
		for /* do {} while(ch != '\n') */ {
			if bpin > eob {
				n, err := r.Read(buf[:len(buf)-1])
				if err != nil && err != io.EOF {
					totalNewline = newlines
					w.Flush()
					return 1
				}
				if err == io.EOF {
					totalNewline = newlines
					w.Flush()
					return 0
				}
				eob = n
				buf[eob] = 10
			} else {
				newlines++
				if newlines > 0 {
					if newlines >= 2 {
						newlines = 2
						if *squeeze {
							ch = buf[bpin]
							bpin++
							continue
						}
					}
					if *number && !*blank {
						nextLineNum()
						w.Write(LineBuf[LinePrint:])
					}
				}
				if *ends {
					w.Write(LineTerm)
				}

				w.WriteByte(10)
			}
			ch = buf[bpin]
			bpin++

			if ch != 10 {
				break
			}
		}

		if newlines >= 0 && *number {
			nextLineNum()
			w.Write(LineBuf[LinePrint:])
		}

		if showNonPrinting {
			// Theoretically this could be if/else statements
			// instead of a switch. Performance will be tested
			// in an upcoming version.
		Outer:
			for {
				switch ch {
				// catch '\n' early
				case 10:
					newlines = -1
					break Outer
				case 0:
					w.Write(Null)
				case 4:
					w.Write(Eot)
				case 7:
					w.Write(Bell)
				case 8:
					w.Write(Backspace)
				case 9:
					if *tabs {
						w.Write(HorizTab)
					} else {
						w.WriteByte(ch)
					}
				case 11:
					w.Write(VertTab)
				case 12:
					w.Write(FormFeed)
				case 13:
					w.Write(Return)
				case 27:
					w.Write(Escape)
				case 127:
					w.Write(Delete)
				default:
					w.WriteByte(ch)
				}
				ch = buf[bpin]
				bpin++
			}
		} else {
			for {
				if ch == 9 && *tabs {
					w.Write(HorizTab)
				} else if ch != 10 {
					w.WriteByte(ch)
				} else {
					newlines = -1
					break
				}
				ch = buf[bpin]
				bpin++
			}
		}
	}

	w.Flush()
	return ok
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
	if *blank {
		*number = true
	}
	if *npTabs {
		*tabs = true
	}
	if *all || *npEnds || *npTabs || *nonPrint {
		showNonPrinting = true
	}

	outStat, err := os.Stdout.Stat()
	if err != nil {
		fatal.Fatalln(err)
	}
	outReg := outStat.Mode().IsRegular()
	outBsize := int(outStat.Sys().(*syscall.Stat_t).Size)

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

		// prfetch! prefetch! prefetch!
		posix.Fadvise(file, 0, 0, posix.POSIX_FADV_SEQUENTIAL)

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
			size := max(inBsize, outBsize)
			outBuf := bufio.NewWriterSize(os.Stdout, size)
			ok ^= simpleCat(file, outBuf)
			outBuf.Flush()
		} else {
			size := 20 + inBsize*4
			outBuf := bufio.NewWriterSize(os.Stdout, size)
			inBuf := make([]byte, inBsize+1)
			ok ^= cat(file, inBuf, outBuf)
		}

		file.Close()
	}

	os.Exit(ok)
}
