package wc

import (
	"bytes"
	"io"
	"os"
	"unicode"
	"unicode/utf8"

	"github.com/ericlagergren/go-coreutils/wc/internal/sys"
)

type Results struct {
	Lines     int64
	Words     int64
	Chars     int64
	Bytes     int64
	MaxLength int64
}

type Counter struct {
	TabWidth int64

	buf  [1 << 17]byte
	opts uint8
}

const (
	Lines     = 1 << iota // count lines
	Words                 // count words
	Chars                 // count chars
	Bytes                 // count bytes
	MaxLength             // find max line length
)

func NewCounter(opts uint8) *Counter {
	return &Counter{opts: opts, TabWidth: 8}
}

func (c *Counter) read(r io.Reader) (int64, error) {
	n, err := r.Read(c.buf[:])
	if err != nil {
		return 0, err
	}
	return int64(n), nil
}

var newLine = []byte{'\n'}

func (c *Counter) Count(r io.Reader) (res Results, err error) {
	if file, ok := r.(*os.File); ok {
		if c.opts == Bytes {
			if n, ok := statSize(file); ok {
				return Results{Bytes: n}, nil
			}
		}
		sys.Fadvise(int(file.Fd()))
	}
	switch c.opts {
	case Bytes:
		for {
			n, err := c.read(r)
			res.Bytes += n
			if err != nil {
				if err == io.EOF {
					return res, nil
				}
				return res, err
			}
		}
	case Lines, Lines | Bytes:
		for {
			n, err := c.read(r)
			res.Bytes += n
			res.Lines += int64(bytes.Count(c.buf[:n], newLine))
			if err != nil {
				if err == io.EOF {
					return res, nil
				}
				return res, err
			}
		}
	default:
		return c.countComplicated(r)
	}
}

func (c *Counter) countComplicated(r io.Reader) (res Results, err error) {
	var (
		pos    int64
		inword int64
	)
	for {
		n, err := c.read(r)
		res.Bytes += n
		if err != nil {
			if err == io.EOF {
				break
			}
			return res, err
		}

		for bp := 0; int64(bp) < n; {
			r, s := utf8.DecodeRune(c.buf[bp:])
			switch r {
			case '\n':
				res.Lines++
				fallthrough
			case '\r':
				fallthrough
			case '\f':
				if pos > res.MaxLength {
					res.MaxLength = pos
				}
				pos = 0
				res.Words += inword
				inword = 0
			case '\t':
				pos += c.TabWidth - (pos % c.TabWidth)
				res.Words += inword
				inword = 0
			case ' ':
				pos++
				fallthrough
			case '\v':
				res.Words += inword
				inword = 0
			default:
				if !unicode.IsPrint(r) {
					break
				}

				pos++
				if unicode.IsSpace(r) {
					res.Words += inword
					inword = 0
				} else {
					inword = 1
				}
			}
			res.Chars++
			bp += s
		}
	}
	if pos > res.MaxLength {
		res.MaxLength = pos
	}
	res.Words += inword
	return res, nil
}

func statSize(file *os.File) (n int64, ok bool) {
	stat, err := file.Stat()
	if err != nil {
		return 0, false
	}

	const badMode = os.ModeDir | os.ModeNamedPipe | os.ModeSocket
	if stat.Mode()&badMode != 0 {
		return 0, false
	}

	// NOTE(eric): GNU's wc says we should seek 1 block size from EOF because it
	// works better on proc-like systems. I like the idea, but I don't want this
	// code to be under the GPL.

	return stat.Size(), true
}
