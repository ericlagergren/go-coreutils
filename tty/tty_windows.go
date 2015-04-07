/*
	Go tty -- print the name of the terminal connected to standard input

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

/* Written by Eric Lagergren */

package main

import (
	"bytes"
	"fmt"
	"os"
	"syscall"

	"github.com/EricLagerg/go-gnulib/windows"
	flag "github.com/ogier/pflag"
)

const (
	VERSION = `tty (Go coreutils) 1.0
Copyright (C) 2015 Eric Lagergren
License GPLv3+: GNU GPL version 3 or later <http://gnu.org/licenses/gpl.html>.
This is free software: you are free to change and redistribute it.
There is NO WARRANTY, to the extent permitted by law.

Written by Eric Lagergren
Inspired by David MacKenzie.`
	HELP = `Usage: tty [OPTION]...
Print the file name of the terminal connected to standard input.

  -s, --silent, --quiet   print nothing, only return an exit status
      --help     display this help and exit
      --version  output version information and exit

Report uname bugs to ericscottlagergren@gmail.com
Go coreutils home page: <https://www.github.com/EricLagerg/go-coreutils/>`
)

var (
	version = flag.Bool("version", false, "print version")
	quiet1  = flag.BoolP("silent", "s", false, "no output")
	quiet2  = flag.Bool("quiet", false, "no output")
)

func count(s []byte) int64 {
	count := 0
	i := 0
	for i < len(s) {
		if s[i] != delim {
			o := bytes.IndexByte(s[i:], 0)
			if o < 0 {
				break
			}
			i += o
		}
		count++
		i++
	}
	return count
}

func main() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "%s\n", HELP)
		return
	}
	flag.Parse()

	if *version {
		fmt.Printf("%s\n", VERSION)
		return
	}

	silent := *quiet1 || *quiet2

	buf := make([]byte, syscall.MAX_PATH)
	err := k32.GetConsoleTitleA(buf)
	if err != nil {
		fmt.Println("not a tty")
		if !silent {
			os.Exit(1)
		}
	}

	// All null bytes, so no name, but err == nil so it's still a tty, it just
	// doesn't have a name.
	if count(buf) == syscall.MAX_PATH {
		fmt.Println("tty")
		return
	}

	fmt.Printf("%s\n", buf)
}
