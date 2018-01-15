package wc

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"math"
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

	if opts == 0 {
		opts = Lines | Words | Bytes
	}

	ctr := NewCounter(opts)
	ctr.TabWidth = c.tabWidth

	var s interface {
		Scan() bool
		Text() string
	}
	var hint int // To keep from allocating, if possible.
	if c.filesFrom == "" {
		if c.f.NArg() == 0 {
			res, err := ctr.Count(ctx.Stdin)
			if err != nil {
				fmt.Fprintln(ctx.Stderr, err)
				return err
			}
			width := 7
			if opts&(opts-1) == 0 { // power of 2, so 1 argument set.
				width = 1
			}
			writeCounts(ctx.Stdout, width, opts, res, "")
			return nil
		}
		s = &sliceScanner{s: c.f.Args()}
		hint = c.f.NArg()
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

	var (
		results  = make([]Results, 0, hint)
		names    = make([]string, 0, hint)
		total    Results
		maxBytes int64
		minWidth = 1
	)

	for s.Scan() {
		fname := s.Text()

		file, err := os.Open(fname)
		if err != nil {
			fmt.Fprintln(ctx.Stderr, err)
			return err
		}

		stat, err := file.Stat()
		if err != nil || (err == nil && !stat.Mode().IsRegular()) {
			minWidth = 7
		}

		res, err := ctr.Count(file)
		if err != nil {
			fmt.Fprintln(ctx.Stderr, err)
			return err
		}
		results = append(results, res)
		names = append(names, fname)

		total.Lines += res.Lines
		total.Words += res.Words
		total.Chars += res.Chars
		total.Bytes += res.Bytes

		if res.Bytes > maxBytes {
			maxBytes = res.Bytes
		}

		if res.MaxLength > total.MaxLength {
			total.MaxLength = res.MaxLength
		}

		if err := file.Close(); err != nil {
			fmt.Fprintln(ctx.Stderr, err)
			return err
		}
	}

	// Fast integer log 10. The call to math.Pow and subsequent comparison can
	// be dropped in favor of simply adding +1 to width if it's alright for the
	// result to be +1 too large for some numbers.
	width := int((bits.Len64(uint64(maxBytes)) * 1233) >> 12)
	if int64(math.Pow10(width)) < maxBytes {
		width++
	}
	if width < minWidth {
		width = minWidth
	}
	for i, r := range results {
		writeCounts(ctx.Stdout, width, opts, r, names[i])
	}
	if len(results) > 1 {
		writeCounts(ctx.Stdout, width, opts, total, "total")
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
