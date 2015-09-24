/*
    go md5sum

    Copyright (c) 2014-2015 Dingjun Fang

	This program is free software: you can redistribute it and/or modify
	it under the terms of the GNU General Public License version 3 as
	published by the Free Software Foundation.

	This program is distributed in the hope that it will be useful,
	but WITHOUT ANY WARRANTY; without even the implied warranty of
	MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
	GNU General Public License for more details.

	You should have received a copy of the GNU General Public License
	along with this program.  If not, see <http://www.gnu.org/licenses/>.
*/

/*

Md5sum util implement by go.

Usage: md5sum [OPTION] [FILE]...

or: md5sum [OPTION] --check [FILE]

Print or check MD5 checksums.

With no FILE, or when FILE is -, read standard input.
  -c, --check   check MD5 sums against given list

The following two options are useful only when verifying checksums:
      --status   don't output anything, status code shows success
  -w, --warn     warn about improperly formated checksum lines
      --help     show help and exit
      --version  show version and exit
*/
package main

import (
	"fmt"
	flag "github.com/ogier/pflag"
	"os"
	//"path/filepath"
)

const (
	Help = `Usage: md5sum [OPTION] [FILE]...
   or: md5sum [OPTION] --check [FILE]
Print or check MD5 checksums.
With no FILE, or when FILE is -, read standard input.
  -c, --check   check MD5 sums against given list

The following two options are useful only when verifying checksums:
      --status   don't output anything, status code shows success
  -w, --warn     warn about improperly formated checksum lines
      --help     show help and exit
      --version  show version and exit
`
	Version = `md5sum (Go coreutils) 0.1
Copyright (C) 2015 Dingjun Fang
License GPLv3+: GNU GPL version 3 or later <http://gnu.org/licenses/gpl.html>.
This is free software: you are free to change and redistribute it.
There is NO WARRANTY, to the extent permitted by law.
`
)

var (
	check_sum    = flag.BoolP("check", "c", false, "")
	no_output    = flag.BoolP("status", "", false, "")
	show_warn    = flag.BoolP("warn", "w", true, "")
	show_version = flag.BoolP("version", "v", false, "")
)

func main() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "%s", Help)
		os.Exit(1)
	}

	flag.Parse()

	has_error := false

	switch {
	case *show_version:
		fmt.Fprintf(os.Stdout, "%s", Version)
		os.Exit(0)
	case *check_sum:
		if !check_md5sum() {
			has_error = true
		}
	default:
		if !gen_md5sum() {
			has_error = true
		}
	}

	if has_error {
		os.Exit(1)
	}

	os.Exit(0)
}
