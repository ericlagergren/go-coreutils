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

// usage and version
const (
	Help = `Usage:
       xxd [options] [infile [outfile]]
    or
       xxd -r [-s offset] [-c cols] [--ps] [infile [outfile]]
Options:
    -a, --autoskip     toggle autoskip: A single '*' replaces nul-lines. Default off.
    -B, --bars         print pipes/bars before/after ASCII/EBCDIC output. Default off.
    -b, --binary       binary digit dump (incompatible with -ps, -i, -r).Default hex.
    -c, --cols         format <cols> octets per line. Default 16 (-i 12, --ps 30).
    -E, --ebcdic       show characters in EBCDIC. Default ASCII.
    -g, --groups       number of octets per group in normal output. Default 2.
    -h, --help         print this summary.
    -i, --include      output in C include file style.
    -l, --length       stop after <len> octets.
    -p, --ps           output in postscript plain hexdump style.
    -r, --reverse      reverse operation: convert (or patch) hexdump into ASCII output.
                       * reversing non-hexdump formats require -r<flag> (i.e. -rb, -ri, -rp).
    -s, --seek         start at <seek> bytes/bits in file. Byte/bit postfixes can be used.
    		       * byte/bit postfix units are multiples of 1024.
    		       * bits (kb, mb, etc.) will be rounded down to nearest byte.
    -u, --uppercase    use upper case hex letters.
    -v, --version      show version.`
	Version = `xxd v2.0 2014-17-01 by Felix Geisendörfer and Eric Lagergren`
)

// cli flags
var (
	autoskip   = flag.BoolP("autoskip", "a", false, "toggle autoskip (* replaces nul lines")
	bars       = flag.BoolP("bars", "B", false, "print |ascii| instead of ascii")
	binary     = flag.BoolP("binary", "b", false, "binary dump, incompatible with -ps, -i, -r")
	columns    = flag.IntP("cols", "c", -1, "format <cols> octets per line")
	ebcdic     = flag.BoolP("ebcdic", "E", false, "use EBCDIC instead of ASCII")
	group      = flag.IntP("group", "g", -1, "num of octets per group")
	cfmt       = flag.BoolP("include", "i", false, "output in C include format")
	length     = flag.Int64P("len", "l", -1, "stop after len octets")
	postscript = flag.BoolP("ps", "p", false, "output in postscript plain hd style")
	reverse    = flag.BoolP("reverse", "r", false, "convert hex to binary")
	offset     = flag.Int("off", 0, "revert with offset")
	seek       = flag.StringP("seek", "s", "", "start at seek bytes abs")
	upper      = flag.BoolP("uppercase", "u", false, "use uppercase hex letters")
	version    = flag.BoolP("version", "v", false, "print version")
)

// constants used in xxd()
const (
	ebcdicOffset = 0x40
)

// dumpType enum
const (
	dumpHex = iota
	dumpBinary
	dumpCformat
	dumpPostscript
)

// variables used in xxd*()
var (
	dumpType int

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
	bar          = []byte("|")
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

// returns -1 on success
// returns k > -1 if space found where k is index of space byte
func binaryDecode(dst, src []byte) int {
	var d byte

	for i, v := range src {
		d <<= 1
		if isSpace(&v) { // found a space, so between groups
			if i == 0 {
				return 1
			}
			return i
		}
		if v == '1' {
			d ^= 1
		} else if v != '0' {
			return i // will catch issues like "000000: "
		}
	}

	dst[0] = d
	return -1
}

// hex lookup table for hex encoding
const (
	ldigits = "0123456789abcdef"
	udigits = "0123456789ABCDEF"
)

func cfmtEncode(dst, src []byte, hextable string) {
	dst[0] = '0'
	dst[1] = 'x'
	for i, v := range src {
		dst[i+1*2] = hextable[v>>4]
		dst[i+1*2+1] = hextable[v&0x0f]
	}
}

// copied from encoding/hex package in order to add support for uppercase hex
func hexEncode(dst, src []byte, hextable string) {
	for i, v := range src {
		dst[i*2] = hextable[v>>4]
		dst[i*2+1] = hextable[v&0x0f]
	}
}

// copied from encoding/hex package
// returns -1 on bad byte or space (\t \s \n)
// returns -2 on two consecutive spaces
// returns 0 on success
func hexDecode(dst, src []byte) int {
	if isSpace(&src[0]) {
		if isSpace(&src[1]) {
			return -2
		}
		return -1
	}

	if isPrefix(src[0:2]) {
		src = src[2:]
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

		dst[0] = (a << 4) | b
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
func empty(b []byte) bool {
	for _, v := range b {
		if v != 0 {
			return false
		}
	}
	return true
}

// quick binary tree check
// probably horribly written idk it's late at night
func parseSpecifier(b string) float64 {
	var b0, b1 byte
	lb := len(b)

	if lb < 2 {
		if lb == 0 {
			return 0
		}
		b0 = b[0]
		b1 = '0'
	} else {
		b0 = b[0]
		b1 = b[1]
	}

	if b1 != '0' {
		if b1 == 'b' { // bits, so convert bytes to bits for os.Seek()
			if b0 == 'k' || b0 == 'K' {
				return 0.0078125
			}

			if b0 == 'm' || b0 == 'M' {
				return 7.62939453125e-06
			}

			if b0 == 'g' || b0 == 'G' {
				return 7.45058059692383e-09
			}
		}

		if b1 == 'B' { // kilo/mega/giga- bytes are assumed
			if b0 == 'k' || b0 == 'K' {
				return 1024
			}

			if b0 == 'm' || b0 == 'M' {
				return 1048576
			}

			if b0 == 'g' || b0 == 'G' {
				return 1073741824
			}
		}
	} else { // kilo/mega/giga- bytes are assumed for single b, k, m, g
		if b0 == 'k' || b0 == 'K' {
			return 1024
		}

		if b0 == 'm' || b0 == 'M' {
			return 1048576
		}

		if b0 == 'g' || b0 == 'G' {
			return 1073741824
		}
	}

	return 1 // assumes bytes as fallback
}

// parses *seek input
func parseSeek(s string) int64 {
	var (
		sl    = len(s)
		split int
	)

	if sl >= 2 {
		if sl == 2 {
			split = 1
		} else {
			split = 2
		}
	} else if sl != 0 {
		split = 0
	} else {
		log.Fatalln("seek string somehow has len of 0")
	}

	mod := parseSpecifier(s[sl-split:])

	ret, err := strconv.ParseFloat(s[:sl-split], 64) //64 bit float
	if err != nil {
		log.Fatalln(err)
	}

	return int64(ret * mod)
}

// is byte a space? (\t, \n, \s)
func isSpace(b *byte) bool {
	return *b == 32 || *b == 9 || *b == 12
}

// are the two bytes hex prefixes? (0x or 0X)
func isPrefix(b []byte) bool {
	return b[0] == '0' && (b[1] == 'x' || b[1] == 'X')
}

func xxdReverse(r io.Reader, w io.Writer) error {
	var (
		cols int
		octs int
		char = make([]byte, 1)
	)

	if *columns != -1 {
		cols = *columns
	}

	switch dumpType {
	case dumpBinary:
		octs = 8
	case dumpCformat:
		octs = 4
	default:
		octs = 2
	}

	if *length != -1 {
		if *length < int64(cols) {
			cols = int(*length)
		}
	}

	if octs < 1 {
		octs = cols
	}

	c := int64(0) // number of characters
	rd := bufio.NewReader(r)
	for {
		line, err := rd.ReadBytes('\n') // read up until a newline
		n := len(line)
		if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
			return err
		}

		if n == 0 {
			return nil
		}

		if dumpType == dumpHex {
			for i := 0; n >= octs; {
				if rv := hexDecode(char, line[i:i+octs]); rv == 0 {
					w.Write(char)
					i += 2
					n -= 2
					c++
				} else if rv == -1 {
					i++
					n--
				} else { // if rv == -2
					i += 2
					n -= 2
				}
			}
		} else if dumpType == dumpBinary {
			for i := 0; n >= octs; {
				if binaryDecode(char, line[i:i+octs]) != -1 {
					i++
					n--
					continue
				} else {
					w.Write(char)
					i += 8
					n -= 8
					c++
				}
			}
		} else if dumpType == dumpPostscript {
			for i := 0; n >= octs; i++ {
				if hexDecode(char, line[i:i+octs]) == 0 {
					w.Write(char)
					c++
				}
				n--
			}
		} else if dumpType == dumpCformat {
			for i := 0; n >= octs; {
				if rv := hexDecode(char, line[i:i+octs]); rv == 0 {
					w.Write(char)
					i += 4
					n -= 4
					c++
				} else if rv == -1 {
					i++
					n--
				} else { // if rv == -2
					i += 2
					n -= 2
				}
			}
		}

		// For some reason "xxd FILE | xxd -r -c N" truncates the output,
		// so we'll do it as well
		// "xxd FILE | xxd -r -l N" doesn't truncate
		if c == int64(cols) && cols > 0 {
			return nil
		}
	}
	return nil
}

func xxd(r io.Reader, w io.Writer, fname string) error {
	var (
		lineOffset int64
		hexOffset  = make([]byte, 6)
		groupSize  int
		cols       int
		octs       int
		caps       = ldigits
		doCHeader  = true
		doCEnd     bool
		// enough room for "unsigned char NAME_FORMAT[] = {"
		varDeclChar = make([]byte, 14+len(fname)+6)
		// enough room for "unsigned int NAME_FORMAT = "
		varDeclInt = make([]byte, 16+len(fname)+7)
		nulLine    int64
		totalOcts  int64
	)

	// Generate the first and last line in the -i output:
	// e.g. unsigned char foo_txt[] = { and unsigned int foo_txt_len =
	if dumpType == dumpCformat {
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
		switch dumpType {
		case dumpPostscript:
			cols = 30
		case dumpCformat:
			cols = 12
		case dumpBinary:
			cols = 6
		default:
			cols = 16
		}
	} else {
		cols = *columns
	}

	// See above comment
	switch dumpType {
	case dumpBinary:
		octs = 8
		groupSize = 1
	case dumpPostscript:
		octs = 0
	case dumpCformat:
		octs = 4
	default:
		octs = 2
		groupSize = 2
	}

	if *group != -1 {
		groupSize = *group
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
	// allow their sizes to be allocated based on the user's speficiations
	var (
		line = make([]byte, cols)
		char = make([]byte, octs)
	)

	c := int64(0) // number of characters
	nl := int64(0)
	r = bufio.NewReader(r)
	for {
		n, err := io.ReadFull(r, line)
		if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
			return err
		}

		// Speed it up a bit ;)
		if dumpType == dumpPostscript && n != 0 {
			// Post script values
			// Basically just raw hex output
			for i := 0; i < n; i++ {
				hexEncode(char, line[i:i+1], caps)
				w.Write(char)
				c++
			}
			continue
		}

		if n == 0 {
			if dumpType == dumpPostscript {
				w.Write(newLine)
			}

			if dumpType == dumpCformat {
				doCEnd = true
			} else {
				return nil // Hidden return!
			}
		}

		if *length != -1 {
			if totalOcts == *length {
				break
			}
			totalOcts += *length
		}

		if *autoskip && empty(line) {
			if nulLine == 1 {
				w.Write(asterisk)
				w.Write(newLine)
			}

			nulLine++

			if nulLine > 1 {
				lineOffset += int64(n) // continue to increment our offset
				continue
			}
		}

		if dumpType <= dumpBinary { // either hex or binary
			// Line offset
			hexOffset = strconv.AppendInt(hexOffset[0:0], lineOffset, 16)
			w.Write(zeroHeader[0:(7 - len(hexOffset))])
			w.Write(hexOffset)
			w.Write(zeroHeader[7:])
			lineOffset += int64(n)
		} else if doCHeader {
			w.Write(varDeclChar)
			w.Write(newLine)
			doCHeader = false
		}

		if dumpType == dumpBinary {
			// Binary values
			for i, k := 0, octs; i < n; i, k = i+1, k+octs {
				binaryEncode(char, line[i:i+1])
				w.Write(char)
				c++

				if k == octs*groupSize || i == cols-1 {
					k = 0
					w.Write(space)
				}
			}
		} else if dumpType == dumpCformat {
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
		} else {
			// Hex values -- default xxd FILE output
			for i, k := 0, octs; i < n; i, k = i+1, k+octs {
				hexEncode(char, line[i:i+1], caps)
				w.Write(char)
				c++

				if k == octs*groupSize || i == cols-1 {
					k = 0 // reset counter
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

		// If we didn't read a full line, determine our position
		// and fill the rest of the line with spaces.
		if n < cols && dumpType <= dumpBinary {

			lineLen := cols*octs + ((cols * octs) / (octs * groupSize))
			pos := n*octs + ((n * octs) / (octs * groupSize))
			for i := pos; i < lineLen; i++ {
				w.Write(space)
			}
		}

		if dumpType != dumpCformat {
			w.Write(space)
		}

		if dumpType <= dumpBinary {
			// Character values
			b := line[:n]
			// |hello, world!| instead of hello, world!
			if *bars {
				w.Write(bar)
			}
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
				if *bars {
					w.Write(bar)
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
			if *bars {
				w.Write(bar)
			}
		}
		w.Write(newLine)
		nl++
	}
	return nil
}

func main() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "%s\n", Help)
		os.Exit(0)
	}
	flag.Parse()

	if flag.NArg() == 0 {
		fmt.Fprintf(os.Stderr, "no input file given\n%s\n", Help)
		os.Exit(1)
	}

	if *version {
		fmt.Fprintf(os.Stderr, "%s\n", Version)
		os.Exit(0)
	}

	if flag.NArg() > 2 {
		log.Fatalf("too many arguments after %s\n", flag.Arg(1))
	}

	var (
		err  error
		file string
	)

	if flag.NArg() >= 1 {
		file = flag.Arg(0)
	} else {
		file = "-"
	}

	var inFile *os.File
	if file == "-" {
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
	if *seek != "" {
		sv := parseSeek(*seek)
		_, err := inFile.Seek(sv, os.SEEK_SET)
		if err != nil {
			log.Fatalln(err)
		}
	}

	var outFile *os.File
	if flag.NArg() == 2 {
		outFile, err = os.OpenFile(flag.Arg(1), os.O_RDWR|os.O_CREATE, 0660)
		if err != nil {
			log.Fatalln(err)
		}
	} else {
		outFile = os.Stdout
	}
	defer outFile.Close()

	switch true {
	case *binary:
		dumpType = dumpBinary
	case *cfmt:
		dumpType = dumpCformat
	case *postscript:
		dumpType = dumpPostscript
	default:
		dumpType = dumpHex
	}

	out := bufio.NewWriter(outFile)
	defer out.Flush()

	if *reverse {
		if err := xxdReverse(inFile, out); err != nil {
			log.Fatalln(err)
		}
		return
	} else {
		if err := xxd(inFile, out, file); err != nil {
			log.Fatalln(err)
		}
		return
	}
}
