package main

import (
	"bufio"
	"bytes"
	"crypto/md5"
	"fmt"
	"io"
	"log"
	"os"

	"github.com/EricLagerg/go-gnulib/posix"
	"github.com/EricLagerg/go-gnulib/ttyname"
	flag "github.com/ogier/pflag"
)

const (
	Help = `Usage: md5sum [OPTION]... [FILE]...
Print or check MD5 (128-bit) checksums.
With no FILE, or when FILE is -, read standard input.

  -b, --binary         read in binary mode
  -c, --check          read MD5 sums from the FILEs and check them
      --tag            create a BSD-style checksum
  -t, --text           read in text mode (default)

The following four options are useful only when verifying checksums:
      --quiet          don't print OK for each successfully verified file
      --status         don't output anything, status code shows success
      --strict         exit non-zero for improperly formatted checksum lines
  -w, --warn           warn about improperly formatted checksum lines

      --help     display this help and exit
      --version  output version information and exit

The sums are computed as described in RFC 1321.  When checking, the input
should be a former output of this program.  The default mode is to print
a line with checksum, a character indicating input mode ('*' for binary,
space for text), and name for each FILE.
`

	Version = `Go md5sum (Go coreutils) 2.0
Copyright (C) 2015 Eric Lagergren
License GPLv3+: GNU GPL version 3 or later <http://gnu.org/licenses/gpl.html>.
This is free software: you are free to change and redistribute it.
There is NO WARRANTY, to the extent permitted by law.
`
)

const (
	NameLen      = 3               // len("MD5") for checking BSD style sums
	NumBits      = 128             // number of bits per digest (128 bits)
	NumHexBytes  = NumBits / 4     // number chars per digest (32 chars)
	NumBinBytes  = NumBits / 8     // number bytes per digest (16 bytes)
	MinDigestLen = NumHexBytes + 2 // hex digest + blank + min filename len
)

var (
	binary  = flag.BoolP("binary", "b", false, "")
	check   = flag.BoolP("check", "c", false, "")
	tag     = flag.BoolP("tag", "g", false, "")
	text    = flag.BoolP("text", "t", false, "")
	quiet   = flag.BoolP("quiet", "q", false, "")
	status  = flag.BoolP("status", "s", false, "")
	strict  = flag.BoolP("strict", "i", false, "")
	warn    = flag.BoolP("warn", "w", false, "")
	version = flag.BoolP("version", "v", false, "")

	// fatal = log.New(os.Stderr, "", 0)
	fatal = log.New(os.Stderr, "", log.Lshortfile)

	// BSD checksum
	bsdReversed = -1

	EscapeSlash = []byte{'\\'}
	LineFeed    = []byte{'\n'}
)

func unescapeFilename(b []byte) int {
	for i := 0; i < len(b); i++ {
		switch b[i] {
		case '\\':
		}
	}

	return 1
}

func bsdSplitCheck() int { return 1 }

func splitCheck(name []byte, binaryFile *int) int {
	escape := false

	// Skip past beginning white space. E.g.,
	// The difference between:
	//     d41d8cd98f00b204e9800998ecf8427e  thisisatestfile
	// d41d8cd98f00b204e9800998ecf8427e  thisisatestfile165
	i := 0
	for name[i] == ' ' || name[i] == '\t' {
		i++
	}

	if name[i] == '\\' {
		i++
		escape = true
	}

	// 3 == len("md5")
	if len(name)+i == NameLen {
		if name[i+NameLen] == ' ' {
			i++
		}

		if name[i+3] == '(' {
			*binaryFile = 0
			return bsdSplitCheck( /* args */ )
		}
	}

	// Advance one digest size and check if it's followed by a white-space.
	// If not, that's an error.
	i += NumHexBytes
	if name[i] == ' ' || name[i] == '\t' {
		return 1
	}

	// name[i++] = '\0';
	i++
	name[i] = 0

	// Check for BSD's -r format
	if len(name)-i == 1 || (name[i] != ' ' && name[i] != '*') {
		if bsdReversed == 0 {
			return 1
		}
		bsdReversed = 1
	} else if bsdReversed != 1 {
		bsdReversed = 0

		i++
		if name[i] == '*' {
			*binaryFile = 1
		} else {
			*binaryFile = 0
		}
	}

	if escape {
		return unescapeFilename(name[i:])
	}

	return 0
}

func printFilename(name string, escape bool) {
	if !escape {
		os.Stdout.WriteString(name)
		return
	}

	for _, v := range name {
		switch v {
		case '\n':
			os.Stdout.WriteString("\\n")
		case '\\':
			fmt.Print("\\\\")
		default:
			fmt.Printf("%c", v)
		}
	}
}

func digestCheck(name string) int {
	var (
		file *os.File
		err  error
	)

	if name == "-" {
		file = os.Stdin
	} else {
		file, err = os.Open(name)
		if err != nil {
			fatal.Printf("%s", err.Error())
			return 1
		}
		defer file.Close()
	}

	r := bufio.NewReader(file)

	lineNum := 0
	for {
		lineNum++

		line, err := r.ReadBytes('\n')
		if err == io.EOF {
			break
		}

		if err != nil {
			panic(err)
		}

		// Ignore comments
		if line[0] == '#' {
			continue
		}
	}

	return 0
}

func digestFile(name string, binaryFile *int, buf []byte) error {
	var (
		file *os.File
		err  error
	)

	if name == "-" {
		file = os.Stdin

		if *binaryFile < 0 {
			if ttyname.IsAtty(os.Stdin.Fd()) {
				*binaryFile = 0
			} else {
				*binaryFile = 1
			}
		}
	} else {
		file, err = os.Open(name)
		if err != nil {
			return err
		}
		defer file.Close()
	}

	hash := md5.New()

	posix.Fadvise64(int(file.Fd()), 0, 0, posix.FADVISE_SEQUENTIAL)

	_, err = io.Copy(hash, file)
	if err != nil {
		return err
	}
	copy(buf, hash.Sum(nil))

	return nil
}

func main() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "%s", Help)
		os.Exit(1)
	}
	flag.Parse()

	var (
		bin        = -1
		ok         = 0
		binaryFile int
		buf        = make([]byte, NumBinBytes)
	)

	switch true {
	case *binary:
		bin = 1
	case *text:
		bin = 0
	case *tag:
		bin = 1
	}

	if *tag {
		if bin < 1 {
			fatal.Fatalln("--tag does not support --text mode")
		}

		if *check {
			fatal.Fatalln("the --tag option is meaningless when verifying checksums")
		}
	}

	if 0 <= bin && *check {
		fatal.Fatalln("the --binary and --text options are meaningless when verifying checksums")
	}

	if !*check {
		if *status {
			fatal.Fatalln("the --status option is meaningful only when verifying checksums")
		}

		if *warn {
			fatal.Fatalln("the --warn option is meaningful only when verifying checksums")
		}

		if *quiet {
			fatal.Fatalln("the --quiet option is meaningful only when verifying checksums")
		}

		if *strict {
			fatal.Fatalln("the --strict option is meaningful only when verifying checksums")
		}
	}

	for _, name := range flag.Args() {
		if *check {
			ok ^= digestCheck(name)
		} else {

			binaryFile = bin
			if err := digestFile(name, &binaryFile, buf); err != nil {
				ok = 1
			} else {
				// Why import not strings? Even on a 2,000 character string
				// with 1,000 iterations, converting a string to a byte
				// slice and using bytes.Contains is still roughly the
				// same speed as strings.Contains. Since our file name
				// probably won't be that long, there's no benefit to
				// importing strings.
				escape := bytes.Contains([]byte(name), EscapeSlash) ||
					bytes.Contains([]byte(name), LineFeed)

				if *tag {
					if escape {
						os.Stdout.Write(EscapeSlash)
					}

					os.Stdout.WriteString("MD5 (")
					printFilename(name, escape)
					os.Stdout.WriteString(") = ")
				}

				if !*tag && escape {
					os.Stdout.Write(EscapeSlash)
				}

				// Printf with the hex specifier is actually
				// what GNU's md5sum does, oddly enough.
				fmt.Printf("%02x", buf)

				if !*tag {
					os.Stdout.WriteString(" ")

					if binaryFile > 0 {
						os.Stdout.WriteString("*")
					} else {
						os.Stdout.WriteString(" ")
					}

					printFilename(name, escape)
				}
				os.Stdout.Write(LineFeed)
			}
		}
	}

	os.Exit(ok)
}
