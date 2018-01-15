package wc

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"math/bits"
	"os"
	"unicode"

	coreutils "github.com/ericlagergren/go-coreutils"
	flag "github.com/spf13/pflag"
)

func init() {
	coreutils.Register("wc", run)
}

func newCommand() *cmd {
	var c cmd
	c.f.BoolVarP(&c.lines, "lines", "l", false, "print the newline counts")
	c.f.BoolVarP(&c.words, "words", "w", false, "print the word counts")
	c.f.BoolVarP(&c.chars, "chars", "m", false, "print the character counts")
	c.f.BoolVarP(&c.bytes, "bytes", "c", false, "print the byte counts")
	c.f.BoolVarP(&c.maxLength, "max-line-length", "L", false, "print the length of the longest line")
	c.f.StringVar(&c.filesFrom, "files0-from", "", `read input from the files specified by
                             NUL-terminated names in file F;
                             If F is - then read names from standard input`)
	c.f.Int64VarP(&c.tabWidth, "tab", "t", 8, "change the tab width")
	c.f.BoolVarP(&c.unicode, "unicode-version", "u", false, "display unicode version and exit")
	return &c
}

type cmd struct {
	f                                     flag.FlagSet
	lines, words, chars, bytes, maxLength bool
	filesFrom                             string
	tabWidth                              int64
	unicode                               bool
}

var errMixedArgs = errors.New("file operands cannot be combined with --files0-from")

func run(ctx coreutils.Ctx, args ...string) error {
	c := newCommand()

	// TODO(eric): usage

	if err := c.f.Parse(args); err != nil {
		return err
	}

	if c.unicode {
		fmt.Fprintf(ctx.Stdout, "Unicode version: %s\n", unicode.Version)
		return nil
	}

	var opts uint8
	if c.lines {
		opts |= Lines
	}
	if c.words {
		opts |= Words
	}
	if c.chars {
		opts |= Chars
	}
	if c.bytes {
		opts |= Bytes
	}
	if c.maxLength {
		opts |= MaxLength
	}

	ctr := NewCounter(opts)
	ctr.TabWidth = c.tabWidth

	const minWidth = 7 // default width for printing

	var s interface {
		Scan() bool
		Text() string
	}
	if c.filesFrom == "" {
		if c.f.NArg() == 0 {
			res, err := ctr.Count(ctx.Stdin)
			if err != nil {
				fmt.Fprintln(ctx.Stderr, err)
				return err
			}
			// TODO(eric): name? stdin? blank?
			writeCounts(ctx.Stdout, minWidth, opts, res, "-")
			return nil
		}
		s = &sliceScanner{s: c.f.Args()}
	} else {
		if c.f.NArg() > 0 {
			fmt.Fprintln(ctx.Stderr, errMixedArgs)
			return errMixedArgs
		}
		file, err := os.Open(c.filesFrom)
		if err != nil {
			fmt.Fprintln(ctx.Stderr, err)
			return err
		}
		defer file.Close()
		s = bufio.NewScanner(file)
		s.(*bufio.Scanner).Split(filesFromSplit)
	}

	var rs []Results
	var names []string
	var t Results
	var maxBytes int64

	for s.Scan() {
		fname := s.Text()

		file, err := os.Open(fname)
		if err != nil {
			fmt.Fprintln(ctx.Stderr, err)
			return err
		}

		res, err := ctr.Count(file)
		if err != nil {
			fmt.Fprintln(ctx.Stderr, err)
			return err
		}
		rs = append(rs, res)
		names = append(names, fname)

		t.Lines += res.Lines
		t.Words += res.Words
		t.Chars += res.Chars
		t.Bytes += res.Bytes

		if res.Bytes > maxBytes {
			maxBytes = res.Bytes
		}

		if res.MaxLength > t.MaxLength {
			t.MaxLength = res.MaxLength
		}

		if err := file.Close(); err != nil {
			fmt.Fprintln(ctx.Stderr, err)
			return err
		}
	}

	// Fast integer log 10. Possibly +1 too large, but that's fine.
	// Since this can be hard to read, it's
	//    (((64 - clz(n) + 1) * 1233) >> 12) + 1
	width := int(((64-bits.LeadingZeros64(uint64(maxBytes))+1)*1233)>>12) + 1
	for i, r := range rs {
		writeCounts(ctx.Stdout, width, opts, r, names[i])
	}
	if len(rs) > 1 {
		writeCounts(ctx.Stdout, width, opts, t, "total")
	}
	return nil
}

type sliceScanner struct{ s []string }

func (s *sliceScanner) Scan() bool { return len(s.s) > 0 }
func (s *sliceScanner) Text() string {
	t := s.s[0]
	s.s = s.s[1:]
	return t
}

func filesFromSplit(data []byte, atEOF bool) (adv int, token []byte, err error) {
	if atEOF && len(data) == 0 {
		return 0, nil, nil
	}
	if i := bytes.IndexByte(data, 0x00); i >= 0 {
		return i + 1, dropNull(data[0:i]), nil
	}
	if atEOF {
		return len(data), dropNull(data), nil
	}
	return 0, nil, nil
}

func dropNull(data []byte) []byte {
	if len(data) > 0 && data[len(data)-1] == 0x00 {
		return data[0 : len(data)-1]
	}
	return data
}

func writeCounts(w io.Writer, width int, opts uint8, r Results, fname string) {
	const fmtSpInt = " %*d"

	fmtInt := "%*d"
	if opts&Lines != 0 {
		fmt.Fprintf(w, fmtInt, width, r.Lines)
		fmtInt = fmtSpInt
	}
	if opts&Words != 0 {
		fmt.Fprintf(w, fmtInt, width, r.Words)
		fmtInt = fmtSpInt
	}
	if opts&Chars != 0 {
		fmt.Fprintf(w, fmtInt, width, r.Chars)
		fmtInt = fmtSpInt
	}
	if opts&Bytes != 0 {
		fmt.Fprintf(w, fmtInt, width, r.Bytes)
		fmtInt = fmtSpInt
	}
	if opts&MaxLength != 0 {
		fmt.Fprintf(w, fmtInt, width, r.MaxLength)
	}
	fmt.Fprintf(w, " %s\n", fname)
}
