/*
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

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	flag "github.com/ogier/pflag"
)

const (
	Help = `
Usage: basename NAME [SUFFIX]
  or:  basename OPTION... NAME...
Print NAME with any leading directory components removed.
If specified, also remove a trailing SUFFIX.

Mandatory arguments to long options are mandatory for short options too.
  -a, --multiple       support multiple arguments and treat each as a NAME
  -s, --suffix=SUFFIX  remove a trailing SUFFIX; implies -a
  -z, --zero           end each output line with NUL, not newline
      --help     display this help and exit
      --version  output version information and exit

Examples:
  basename /usr/bin/sort          -> "sort"
  basename include/stdio.h .h     -> "stdio"
  basename -s .h include/stdio.h  -> "stdio"
  basename -a any/str1 any/str2   -> "str1" followed by "str2"

`
	Version = `
basename (Go coreutils) 0.1
Copyright (C) 2015 Robert Deusser
License GPLv3+: GNU GPL version 3 or later <http://gnu.org/licenses/gpl.html>.
This is free software: you are free to change and redistribute it.
There is NO WARRANTY, to the extent permitted by law.

`
)

var (
	multiple = flag.BoolP("multiple", "a", false, "")
	suffix   = flag.StringP("suffix", "s", "", "")
	zero     = flag.BoolP("zero", "z", false, "")
	version  = flag.BoolP("version", "v", false, "")
)

func baseName(path, suffix string, null bool) string {
	dir := filepath.Base(path)

	if strings.HasSuffix(dir, suffix) {
		dir = dir[:len(dir)-len(suffix)]
	}

	if !null {
		dir += "\n"
	}

	return dir
}

func main() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "%s", Help)
		os.Exit(1)
	}
	flag.Parse()

	switch {
	case *version:
		fmt.Fprintf(os.Stdout, "%s", Version)
		os.Exit(0)
	case *multiple:
		for _, v := range flag.Args() {
			dir := baseName(v, *suffix, *zero)
			fmt.Print(dir)
		}
	case *suffix != "":
		// Implies --multiple/-a
		for _, v := range flag.Args() {
			dir := baseName(v, *suffix, *zero)
			fmt.Print(dir)
		}
	default:
		dir := baseName(flag.Args()[0], *suffix, *zero)
		fmt.Print(dir)
	}
}
