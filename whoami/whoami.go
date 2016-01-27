/*
	Go whoami -- print effective userid

	Copyright (c) 2015 Eric Lagergren

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

/* Equivalent to 'id -un'. */
/* Written by Eric Lagergren and mattn */

package main

import (
	"fmt"
	"log"
	"os"

	flag "github.com/ogier/pflag"
)

const (
	Help = `Usage: whoami [OPTION]...
Print the user name associated with the current effective user ID.
Same as id -un.

      --help     display this help and exit
      --version  output version information and exit

Report whoami bugs to ericscottlagergren@gmail.com
Go coreutils home page: <https://www.github.com/EricLagergren/go-coreutils/>`

	Version = `whoami (Go coreutils) 1.0
Copyright (C) 2015 Eric Lagergren
License GPLv3+: GNU GPL version 3 or later <http://gnu.org/licenses/gpl.html>.
This is free software: you are free to change and redistribute it.
There is NO WARRANTY, to the extent permitted by law.

Written by Eric Lagergren`
)

var (
	version = flag.BoolP("version", "v", false, "")
	fatal   = log.New(os.Stderr, "", 0)
)

func main() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "%s", Help)
		os.Exit(0)
	}
	flag.Parse()

	if *version {
		fmt.Fprintf(os.Stderr, "%s", Version)
		os.Exit(0)
	}

	// This function call *should* be inlined on both
	// Windows and Unix
	fmt.Println(getUser())
}
