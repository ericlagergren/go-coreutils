/*
	Go chown -- change ownership of a file

	Copyright (c) 2014-2015  Eric Lagergren

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
	Written by Eric Lagergren <ericscottlagergren@gmail.com>
*/

// BUG(eric): -R flag could get stuck in an infinite loop

package main

import (
	"fmt"
	"os"

	flag "github.com/EricLagerg/pflag"
)

const (
	Help    = "help"
	Version = "version"
)

var (
	recursive      = flag.BoolP("recursive", "R", false, "")
	changes        = flag.BoolP("changes", "c", false, "")
	dereference    = flag.Bool("dereference", false, "")
	from           = flag.Bool("from", false, "")
	noDereference  = flag.BoolP("no-dereference", "h", false, "")
	noPreserveRoot = flag.Bool("no-preserve-root", false, "")
	quiet          = flag.Bool("quiet", false, "")
	silent         = flag.Bool("silent", false, "")
	reference      = flag.Bool("reference", false, "")
	verbose        = flag.BoolP("verbose", "v", false, "")

	version = flag.Bool("version", false, "")
)

func main() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "%s", Help)
		os.Exit(1)
	}
	flag.Parse()

	if *version {
		fmt.Printf("%s\n", Version)
		os.Exit(0)
	}

	// 1 if --dereference, 0 if --no-dereference, -1 if neither
	// has been specified.
	// deref := -1
	// bitFlags := 12 // todo
}
