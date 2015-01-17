package main

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"os"
	"strconv"

	flag "github.com/ogier/pflag"
)

const (
	Help = `Usage:
       xxd [options] [infile [outfile]]
    or
       xxd -r [-s offset] [-c cols] [--ps] [infile [outfile]]
Options:
    -a, --autoskip     toggle autoskip: A single '*' replaces nul-lines. Default off.
    -b, --binary       binary digit dump (incompatible with -ps,-i,-r). Default hex.
    -c, --cols         format <cols> octets per line. Default 16 (-i 12, --ps 30).
    -E, --ebcdic       show characters in EBCDIC. Default ASCII.
    -g, --groups       number of octets per group in normal output. Default 2.
    -h, --help         print this summary.
    -i, --include      output in C include file style.
    -l, --length       stop after <len> octets.
        --ps           output in postscript plain hexdump style.
    -r, --reverse      reverse operation: convert (or patch) hexdump into ASCII output.
    -s, --seek         start at <seek> bytes abs. (or +: rel.) infile offset.
    -u, --uppercase    use upper case hex letters.
    -v, --version      show version.`
	Version = `xxd v1.0 2014-14-01 by Felix Geisendörfer and Eric Lagergren`
)

// cli flags
var (
	autoskip   = flag.BoolP("autoskip", "a", false, "toggle autoskip (* replaces nul lines")
	binary     = flag.BoolP("binary", "b", false, "binary dump, incompatible with -ps, -i, -r")
	columns    = flag.IntP("cols", "c", -1, "format <cols> octets per line")
	ebcdic     = flag.BoolP("ebcdic", "E", false, "use EBCDIC instead of ASCII")
	group      = flag.IntP("group", "g", -1, "num of octets per group")
	cfmt       = flag.BoolP("include", "i", false, "output in C include format")
	length     = flag.Int64P("len", "l", -1, "stop after len octets")
	postscript = flag.Bool("ps", false, "output in postscript plain hd style")
	reverse    = flag.BoolP("reverse", "r", false, "convert hex to binary")
	offset     = flag.Int("off", 0, "revert with offset")
	seek       = flag.Int64P("seek", "s", 0, "start at seek bytes abs")
	upper      = flag.BoolP("uppercase", "u", false, "use uppercase hex letters")
	version    = flag.BoolP("version", "v", false, "print version")
)

// constants used in xxd()
const (
	ebcdicOffset = 0x40
)

// variables used in xxd()
var (
	space        = []byte(" ")
	doubleSpace  = []byte("  ")
	dot          = []byte(".")
	newLine      = []byte("\n")
	zeroHeader   = []byte("0000000: ")
	unsignedChar = []byte("unsigned char ")
	unsignedInt  = []byte("};\nunsigned int ")
	lenEquals    = []byte("_len = ")
	brackets     = []byte("[] = {")
	asterisk     = []byte("*")
	hexPrefix    = []byte("0x")
	commaSpace   = []byte(", ")
	comma        = []byte(",")
	semiColonNl  = []byte(";\n")
)

// ascii -> ebcdic lookup table
var ebcdicTable = []byte{
	0040, 0240, 0241, 0242, 0243, 0244, 0245, 0246,
	0247, 0250, 0325, 0056, 0074, 0050, 0053, 0174,
	0046, 0251, 0252, 0253, 0254, 0255, 0256, 0257,
	0260, 0261, 0041, 0044, 0052, 0051, 0073, 0176,
	0055, 0057, 0262, 0263, 0264, 0265, 0266, 0267,
	0270, 0271, 0313, 0054, 0045, 0137, 0076, 0077,
	0272, 0273, 0274, 0275, 0276, 0277, 0300, 0301,
	0302, 0140, 0072, 0043, 0100, 0047, 0075, 0042,
	0303, 0141, 0142, 0143, 0144, 0145, 0146, 0147,
	0150, 0151, 0304, 0305, 0306, 0307, 0310, 0311,
	0312, 0152, 0153, 0154, 0155, 0156, 0157, 0160,
	0161, 0162, 0136, 0314, 0315, 0316, 0317, 0320,
	0321, 0345, 0163, 0164, 0165, 0166, 0167, 0170,
	0171, 0172, 0322, 0323, 0324, 0133, 0326, 0327,
	0330, 0331, 0332, 0333, 0334, 0335, 0336, 0337,
	0340, 0341, 0342, 0343, 0344, 0135, 0346, 0347,
	0173, 0101, 0102, 0103, 0104, 0105, 0106, 0107,
	0110, 0111, 0350, 0351, 0352, 0353, 0354, 0355,
	0175, 0112, 0113, 0114, 0115, 0116, 0117, 0120,
	0121, 0122, 0356, 0357, 0360, 0361, 0362, 0363,
	0134, 0237, 0123, 0124, 0125, 0126, 0127, 0130,
	0131, 0132, 0364, 0365, 0366, 0367, 0370, 0371,
	0060, 0061, 0062, 0063, 0064, 0065, 0066, 0067,
	0070, 0071, 0372, 0373, 0374, 0375, 0376, 0377,
}

// hex lookup table for hexEncode()
const (
	ldigits = "0123456789abcdef"
	udigits = "0123456789ABCDEF"
)

// copied from encoding/hex package in order to add support for uppercase hex
func hexEncode(dst, src []byte, hextable string) {
	for i, v := range src {
		dst[i*2] = hextable[v>>4]
		dst[i*2+1] = hextable[v&0x0f]
	}
}

func cfmtEncode(dst, src []byte, hextable string) {
	dst[0] = '0'
	dst[1] = 'x'
	for i, v := range src {
		dst[i+1*2] = hextable[v>>4]
		dst[i+1*2+1] = hextable[v&0x0f]
	}
}

// convert a byte into its binary representation
func binaryEncode(dst, src []byte) {
	d := uint(0)
	for i := 7; i >= 0; i-- {
		if src[0]&(1<<d) == 0 {
			dst[i] = '0'
		} else {
			dst[i] = '1'
		}
		d++
	}
}

// copied from encoding/hex package
// returns -1 on bad byte
// returns -2 on space (\n, \t, \s)
// returns -3 on two consecutive spaces
// returns 0 on success
func hexDecode(dst, src []byte) int {
	if src[0] == 32 ||
		src[0] == 9 ||
		src[0] == 12 {
		if src[1] == 32 ||
			src[1] == 9 ||
			src[1] == 12 {
			return -3
		}
		return -2
	}

	for i := 0; i < len(src)/2; i++ {
		a, ok := fromHexChar(src[i*2])
		if !ok {
			return -1
		}
		b, ok := fromHexChar(src[i*2+1])
		if !ok {
			return -1
		}
		dst[i] = (a << 4) | b
	}
	return 0
}

// copied from encoding/hex package
func fromHexChar(c byte) (byte, bool) {
	switch {
	case '0' <= c && c <= '9':
		return c - '0', true
	case 'a' <= c && c <= 'f':
		return c - 'a' + 10, true
	case 'A' <= c && c <= 'F':
		return c - 'A' + 10, true
	}

	return 0, false
}

// check if entire line is full of empty []byte{0} bytes (nul in C)
func empty(b *[]byte) bool {
	for _, v := range *b {
		if v != 0 {
			return false
		}
	}
	return true
}

func xxd(r io.Reader, w io.Writer, fname string) error {
	var (
		lineOffset int64
		hexOffset  = make([]byte, 6)
		caps       = ldigits
		cols       int
		octs       int
		doCHeader  = true
		doCEnd     bool
		// enough room for "unsigned char NAME_FORMAT[] = {"
		varDeclChar = make([]byte, 14+len(fname)+6)
		// enough room for "unsigned int NAME_FORMAT = "
		varDeclInt = make([]byte, 16+len(fname)+7)
		nulLine    int64
		totalOcts  int64
		skip       bool
		odd        = *postscript || *cfmt
		char       []byte
		line       []byte
	)

	if *reverse && (*binary || *cfmt) {
		log.Fatalln("xxd: sorry, cannot revert this type of hexdump")
	}

	// Generate the first and last line in the -i output:
	// e.g. unsigned char foo_txt[] = { and unsigned int foo_txt_len =
	if *cfmt {
		// copy over "unnsigned char " and "unsigned int"
		_ = copy(varDeclChar[0:14], unsignedChar[:])
		_ = copy(varDeclInt[0:16], unsignedInt[:])

		for i := 0; i < len(fname); i++ {
			if fname[i] != '.' {
				varDeclChar[14+i] = fname[i]
				varDeclInt[16+i] = fname[i]
			} else {
				varDeclChar[14+i] = '_'
				varDeclInt[16+i] = '_'
			}
		}
		// copy over "[] = {" and "_len = "
		_ = copy(varDeclChar[14+len(fname):], brackets[:])
		_ = copy(varDeclInt[16+len(fname):], lenEquals[:])
	}

	// Switch between upper- and lower-case hex chars
	if *upper {
		caps = udigits
	}

	// xxd -bpi FILE outputs in binary format
	// xxd -b -p -i FILE outputs in C format
	// simply catch the last option since that's what I assume the author
	// wanted...
	if *columns == -1 {
		switch true {
		case *postscript:
			cols = 30
		case *cfmt:
			cols = 12
		case *binary:
			cols = 6
		default:
			cols = 16
		}
	} else {
		cols = *columns
	}

	// See above comment
	if *group == -1 {
		switch true {
		case *binary:
			octs = 8
		case *postscript:
			octs = 0
		case *cfmt:
			octs = 4
		default:
			octs = 2
		}
	} else {
		octs = *group
	}

	// If -l is smaller than the number of cols just truncate the cols
	if *length != -1 {
		if *length < int64(cols) {
			cols = int(*length)
		}
	}

	if octs < 1 {
		octs = cols
	}

	// These are bumped down from the beginning of the function in order to
	// allow for their sizes to be allocated based on the user's speficiations
	if *reverse {
		line = make([]byte, 9+cols*2+cols)
		char = make([]byte, 1)
	} else {
		line = make([]byte, cols)
		char = make([]byte, octs)
	}

	c := int64(0) // number of characters
	r = bufio.NewReader(r)
	for {
		n, err := io.ReadFull(r, line)
		if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
			return err
		}

		// reverse output of hex files, not binary or C format
		if *reverse && n != 0 {
			// skip first 8 of line because it's the counter thingy
			// don't go to EOL because COLS bytes of that is the human-
			// readable output
			for i := 9; i < n-cols; i += 2 {
				if n := hexDecode(char, line[i:i+2]); n == -1 {
					skip = true
					break
				} else if n == -2 {
					i++ // advance past the space
					hexDecode(char, line[i:i+2])
					w.Write(char)
				} else if n == -3 {
					continue
				} else {
					w.Write(char)
					c++
				}
			}
			if skip {
				skip = false
				continue // we've found a garbage line, so skip it
			}

			// For some reason "xxd FILE | xxd -r -c N" truncates the output,
			// so we'll do it as well
			// "xxd FILE | xxd -r -l N" doesn't truncate
			if c == int64(cols) {
				return nil
			}
			return nil
		}

		// Speed it up a bit ;)
		if *postscript && n != 0 {
			// Post script values
			// Basically just raw hex output
			for i := 0; i < n; i++ {
				hexEncode(char, line[i:i+1], caps)
				w.Write(char)
				c++
			}
			continue
		}

		if n == 0 && !*cfmt {
			if *postscript {
				w.Write(newLine)
			}
			return nil
		} else if n == 0 && *cfmt {
			doCEnd = true
		}

		if *length != -1 {
			if totalOcts == *length {
				break
			}
			totalOcts += *length
		}

		if *autoskip && empty(&line) {
			if nulLine == 1 {
				w.Write(asterisk)
				w.Write(newLine)
			}

			nulLine++

			if nulLine > 1 {
				lineOffset++ // continue to increment our offset
				continue
			}
		}

		if *binary || !odd {
			// Line offset
			hexOffset = strconv.AppendInt(hexOffset[0:0], lineOffset, 16)
			w.Write(zeroHeader[0:(6 - len(hexOffset))])
			w.Write(hexOffset)
			w.Write(zeroHeader[6:])
			lineOffset++
		} else if doCHeader && *cfmt {
			w.Write(varDeclChar)
			w.Write(newLine)
			doCHeader = false
		}

		if *binary {
			// Binary values
			for i := 0; i < n; i++ {
				binaryEncode(char, line[i:i+1])
				w.Write(char)
				w.Write(space)
				c++

			}
		} else if *cfmt {
			// C values
			if !doCEnd {
				w.Write(doubleSpace)
			}
			for i := 0; i < n; i++ {
				cfmtEncode(char, line[i:i+1], caps)
				w.Write(char)
				c++

				// don't add spaces to EOL
				if i != n-1 {
					w.Write(commaSpace)
				} else if doCEnd {
					w.Write(comma)
				}
			}
		} else if !*postscript {
			// Hex values -- default xxd FILE output
			for i := 0; i < n; i++ {
				hexEncode(char, line[i:i+1], caps)
				w.Write(char)
				c++

				if i%2 == 1 {
					w.Write(space)
				}
			}
		}

		if doCEnd {
			w.Write(varDeclInt)
			w.Write([]byte(strconv.FormatInt(c, 10)))
			w.Write(semiColonNl)
			return nil
		}

		if n < len(line) && !*cfmt {
			for i := n; i < len(line); i++ {
				w.Write(doubleSpace)

				if i%2 == 1 {
					w.Write(space)
				}
			}
		}

		if !*cfmt {
			w.Write(space)
		}

		if *binary || !odd {
			// Character values
			b := line[:n]
			// EBCDIC
			if *ebcdic {
				for _, c := range b {
					if c >= ebcdicOffset {
						e := ebcdicTable[c-ebcdicOffset : c-ebcdicOffset+1]
						if e[0] > 0x1f && e[0] < 0x7f {
							w.Write(e)
						} else {
							w.Write(dot)
						}
					} else {
						w.Write(dot)
					}
				}
				// ASCII
			} else {
				for i, c := range b {
					if c > 0x1f && c < 0x7f {
						w.Write(line[i : i+1])
					} else {
						w.Write(dot)
					}
				}
			}
		}
		w.Write(newLine)
	}
	return nil
}

func main() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "%s\n", Help)
		os.Exit(0)
	}
	flag.Parse()

	if *version {
		fmt.Fprintf(os.Stderr, "%s\n", Version)
		os.Exit(0)
	}

	if flag.NArg() > 2 {
		log.Fatalf("too many arguments after %s\n", flag.Args()[1])
	}

	var (
		err  error
		file string
	)

	switch true {
	case *binary:
		*cfmt = false
		*postscript = false
		fallthrough
	case *postscript:
		*postscript = true
		*binary = false
		*cfmt = false
		fallthrough
	case *cfmt:
		*cfmt = true
		*binary = false
		*postscript = false
	default:
		*cfmt = false
		*binary = false
		*postscript = false
	}

	if flag.NArg() >= 1 {
		file = flag.Args()[0]
	} else {
		file = "--"
	}

	var inFile *os.File
	if file == "--" {
		inFile = os.Stdin
		file = "stdin"
	} else {
		inFile, err = os.Open(file)
		if err != nil {
			log.Fatalln(err)
		}
	}
	defer inFile.Close()

	// Start *seek bytes into file
	if *seek != 0 {
		_, err = inFile.Seek(*seek, os.SEEK_SET)
		if err != nil {
			log.Fatalln(err)
		}
	}

	var outFile *os.File
	if flag.NArg() == 2 {
		outFile, err = os.Open(flag.Args()[1])
		if err != nil {
			log.Fatalln(err)
		}
	} else {
		outFile = os.Stdout
	}
	defer outFile.Close()

	out := bufio.NewWriter(outFile)
	defer out.Flush()

	if err := xxd(inFile, out, file); err != nil {
		log.Fatalln(err)
	}
}
