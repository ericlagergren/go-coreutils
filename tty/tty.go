/*
	Go tty -- print the name of the terminal connected to standard input

	Copyright (C) 2015 Eric Lagergren

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

package main

import (
	"fmt"
	"os"

	"github.com/EricLagergren/go-gnulib/ttyname"
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
Go coreutils home page: <https://www.github.com/EricLagergren/go-coreutils/>`
)

var (
	version = flag.Bool("version", false, "print version")
	quiet1  = flag.BoolP("silent", "s", false, "no output")
	quiet2  = flag.Bool("quiet", false, "no output")
)

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

	si := os.Stdin.Fd()
	tty, err := ttyname.TtyName(si)
	if !silent {
		if err == ttyname.NotTty {
			fmt.Println("not a tty")
			os.Exit(1)
		}
		if tty != "" {
			fmt.Println(tty)
			return
		} else {
			fmt.Println("tty")
		}
	}

	if err != nil {
		os.Exit(1)
	}
}
