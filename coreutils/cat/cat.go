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
	By tege@sics.se, Torbjorn Granlund, advised by rms, Richard Stallman
*/

package main

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"os"
	"text/tabwriter"

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
	Version = `Go cat (Go coreutils) 1.0
Copyright (C) 2014 Eric Lagergren
License GPLv3+: GNU GPL version 3 or later <http://gnu.org/licenses/gpl.html>.
This is free software: you are free to change and redistribute it.
There is NO WARRANTY, to the extent permitted by law.

Written by Eric Lagergren <ericscottlagergren@gmail.com>
Inspired by Torbjörn Granlund and Richard M. Stallman.
`
	Caret   = '^'
	NewLine = 10 // \n
)

var (
	inFile *os.File
	err    error
	nlctr  int

	Buffer    = make([]byte, 64*1024)
	OutBuffer = make([]byte, 64*2048)

	all          = flag.BoolP("show-all", "A", false, "equivalent to -vET\n")
	nonBlank     = flag.BoolP("number-nonblank", "b", false, "number nonempty output lines, overrides -n\n")
	nPEnds       = flag.BoolP("nonprinting-ends", "e", false, "equivalent to -vE\n")
	showEnds     = flag.BoolP("show-ends", "E", false, "display $ at end of each line\n")
	number       = flag.BoolP("number", "n", false, "number all output lines\n")
	squeezeBlank = flag.BoolP("squeeze-blank", "s", false, "suppress repeated empty output lines\n")
	nPTabs       = flag.BoolP("nonprinting-tabs", "t", false, "equivalent to -vT\n")
	showTabs     = flag.BoolP("show-tabs", "T", false, "display TAB characters as ^I\n")
	ign          = flag.BoolP("unbuffered", "u", false, "(ignored)\n")
	nP           = flag.BoolP("show-nonprinting", "v", false, "use ^ and M- notation, except for LDF and TAB\ns")

	version   = flag.Bool("version", false, "print version and exit")
	tabWriter = tabwriter.NewWriter(os.Stdout, 3, 0, 2, ' ', tabwriter.AlignRight)
)

func FormatOutput(line []byte, i uint64) {

	// Check if line is a newline, and if so increment our counter
	if len(line) != 0 && len(line) > 2 && line[0] == NewLine {
		nlctr++
	} else {
		// If not, reset it
		nlctr = 0
	}

	// If we've seen a new line and -s is set, skip the next line
	if nlctr > 1 && *squeezeBlank {
		return
		// Print line number for non-blank lines
	} else if *nonBlank && *showEnds || *nPEnds {
		// Any char other than \n on a line with ONE char
		if len(line) == 1 && line[0] != NewLine {
			fmt.Printf("   %d %s$\n", i, line)
			// Anything other than \n on the first space on
			// the line
		} else if line[0] != NewLine {
			fmt.Printf("   %d  %s$\n", i, line[:len(line)-1])
			// Just print the blank line
		} else {
			fmt.Printf("%s$\n", line[:len(line)-1])
		}
	} else if *nonBlank {
		if len(line) == 1 && line[0] != NewLine {
			fmt.Printf("   %d %s\n", i, line)
		} else if line[0] != NewLine {
			fmt.Printf("   %d  %s\n", i, line[:len(line)-1])
		} else {
			fmt.Printf("%s\n", line[:len(line)-1])
		}
		// For numbered lines
	} else if *number {
		if len(line) == 1 && line[0] != NewLine {
			fmt.Printf("   %d %s\n", i, line)
		} else {
			fmt.Printf("   %d  %s\n", i, line[:len(line)-1])
		}
	} else if *showEnds || *all {
		if len(line) == 1 && line[0] == NewLine {
			fmt.Println("$")
		} else if len(line) == 1 && line[0] != NewLine {
			fmt.Printf("%s$\n", line)
		} else {
			fmt.Printf("%s$\n", line[:len(line)-1])
		}
	} else {
		fmt.Printf("%s", line)
	}
}

func Cat(fname string, stdin bool) {
	if stdin {
		inFile = os.Stdin
	} else if fname == "-" {
		inFile = os.Stdin
	} else {
		inFile, err = os.Open(fname)
		if err != nil {
			log.Fatal(err)
		}
		defer inFile.Close()
	}

	bothEnds := *nonBlank && *showEnds || *number && *showEnds
	anyNp := *nPEnds || *nPTabs || *nP

	if *all {
		*nP = true
		*showTabs = true
		*showEnds = true
	}

	// Simple cat -- copy input to output with no formatting
	if !(*number || *showEnds || *showTabs || *nP || *squeezeBlank || *all || *nonBlank || *nPTabs || *nPEnds) {
		for {
			_, err = io.Copy(os.Stdout, inFile)

			if err == nil {
				break
			}
		}
		// For line numbers, line ends, or -s but nothing that changes the
		// content of the strings (e.g. -T, -v)
		//
		// This saves some overhead if we're printing the line as-is, except
		// with line numbers and/or line endings ($)
	} else if !(anyNp || *showTabs) && (bothEnds || *showEnds || *number || *nonBlank || *squeezeBlank) {

		// uint64 instead if int in case we have a file that exceeds
		// 2147483647 lines unlikely, but why not be safe?
		i := uint64(0)
		for {
			inBuffer, err := inFile.Read(Buffer)
			buf := bytes.NewBuffer(Buffer[:inBuffer])

			for {
				line, err := buf.ReadBytes(NewLine)

				// Catch when line is [] (happens at end of files when
				// our buffer is empty for some reason)
				if len(line) == 0 {
					break
				}

				if (bothEnds || *number) ||
					(*nonBlank && len(line) > 1 && line[0] != NewLine) ||
					(i <= 0) {
					i++
				}

				FormatOutput(line, i)

				if err == io.EOF {
					break
				}
			}

			if err != nil {
				break
			}
		}
	} else {
		i := uint64(0)
		for {
			inBuffer, err := inFile.Read(Buffer)
			buf := bytes.NewBuffer(Buffer[:inBuffer])
			c := OutBuffer

			for {
				b, err := buf.ReadByte()
				if err == io.EOF {
					break
				}
				if anyNp || *all {
					if b >= 32 {
						if b < 127 {
							c = append(c, b)
						} else if b == 127 {
							c = append(c, Caret, '?')
						} else {
							c = append(c, 'M', '-')
							if b >= 128+32 {
								if b < 128+127 {
									c = append(c, b-128)
								} else {
									c = append(c, Caret, '?')
								}
							} else {
								c = append(c, Caret, b-128+64)
							}
						}
					} else if b == 9 && !*showTabs {
						c = append(c, 9)
					} else if b == 10 {
						if *number || bothEnds {
							i++
						}
						c = append(c, b)
						FormatOutput(c, i)
						c = c[:0]
					} else {
						c = append(c, Caret, b+64)
					}
				} else {
					if b == 9 && *showTabs {
						c = append(c, Caret, b+64)
					} else {
						c = append(c, b)
					}
				}
			}
			if (bothEnds || *number) ||
				(*nonBlank && len(c) != 0 && c[0] != NewLine) {
				i++
			}

			FormatOutput(c, i)

			if err == io.EOF {
				break
			}

		}
	}

}

func main() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "%s", Help)
		os.Exit(0)
	}
	flag.Parse()
	args := flag.Args()
	succ := true

	if *version {
		fmt.Fprintf(tabWriter, "%s\n", Version)
		tabWriter.Flush()
		os.Exit(0)
	}

	if len(args) > 0 && args[0] != "-" {
		for _, f := range args {
			fi, err := os.Stat(f)
			if err != nil {
				fmt.Println(err)
				succ = false
				continue
			}
			if fi.IsDir() {
				log.Printf("cat: %s: Is a directory\n", f)
				succ = false
			} else {
				Cat(f, false)
			}
		}
	} else {
		Cat("-", true)
	}
	if !succ {
		os.Exit(1)
	}
}
