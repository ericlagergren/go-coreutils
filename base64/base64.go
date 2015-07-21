/*
	Go base64 - prints the current working directory.
	Copyright (C) 2015 Robert Deusser
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
	Written by Robert Deusser <iamthemuffinman@outlook.com>
*/

package main

import (
	"encoding/base64"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"unicode"

	flag "github.com/ogier/pflag"
)

const (
	Help = `
Usage: base64 [OPTION]... [FILE]
Base64 encode or decode FILE, or standard input, to standard output.

Mandatory arguments to long options are mandatory for short options too.
  -d, --decode          decode data
  -i, --ignore-garbage  when decoding, ignore non-alphabet characters
  -w, --wrap=COLS       wrap encoded lines after COLS character (default 76).
                          Use 0 to disable line wrapping

      --help     display this help and exit
      --version  output version information and exit

With no FILE, or when FILE is -, read standard input.

The data are encoded as described for the base64 alphabet in RFC 3548.
When decoding, the input may contain newlines in addition to the bytes of
the formal base64 alphabet.  Use --ignore-garbage to attempt to recover
from any other non-alphabet bytes in the encoded stream.

`
	Version = `
base64 (Go coreutils) 0.1
Copyright (C) 2015 Robert Deusser
License GPLv3+: GNU GPL version 3 or later <http://gnu.org/licenses/gpl.html>.
This is free software: you are free to change and redistribute it.
There is NO WARRANTY, to the extent permitted by law.

`
)

var (
	decode  = flag.BoolP("decode", "d", false, "")
	ignore  = flag.BoolP("ignore-garbage", "i", false, "")
	wrap    = flag.IntP("wrap=COLS", "w", 76, "")
	version = flag.BoolP("version", "v", false, "")
)

func base64Encode(src []byte) []byte {
	return []byte(base64.StdEncoding.EncodeToString(src))
}

func base64Decode(src []byte) ([]byte, error) {
	return base64.StdEncoding.DecodeString(string(src))
}

func readData(reader io.Reader) ([]byte, error) {
	return ioutil.ReadAll(reader)
}

func isAlpha(ch byte) bool {
	return unicode.IsLetter(rune(ch))
}

func readAndHandle(reader io.Reader, decode *bool, ignore *bool, wrap *int) {
	src, err := readData(reader)
	checkError(err)
	var toHandle []byte
	if *ignore {
		//It seems that the effect of "base64 -i" in *nix
		//is not filter the non-alphabet charater.
		//This flag cannot work as the original *nix command flag.
		for i := 0; i < len(src); i++ {
			if isAlpha(src[i]) {
				toHandle = append(toHandle, src[i])
			}
		}
	} else {
		toHandle = src
	}
	if *decode {
		decoded, err := base64Decode(toHandle)
		checkError(err)
		fmt.Printf("%s", string(decoded))
	} else {
		encoded := base64Encode(toHandle)
		wrapPrint(encoded, *wrap)
	}
}

func checkError(err error) {
	if err != nil {
		log.Fatal(err)
	}
}

func wrapPrint(output []byte, wrap int) {
	if wrap == 0 {
		fmt.Printf("%s\n", string(output))
		return
	}

	length := len(output)
	if length <= wrap {
		fmt.Printf("%s\n", string(output))
		return
	}

	index, end := 0, 0
	for index < length {
		end += wrap
		if end > length {
			end = length
		}
		fmt.Printf("%s\n", string(output[index:end]))
		index += wrap
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
	if *wrap < 0 {
		log.Fatalf("invalid wrap size: %d", *wrap)
	}

	if len(flag.Args()) == 0 {
		readAndHandle(os.Stdin, decode, ignore, wrap)
	} else {
		for i := 0; i < len(flag.Args()); i++ {
			file, err := os.Open(flag.Args()[i])
			checkError(err)
			readAndHandle(file, decode, ignore, wrap)
		}
	}
}
