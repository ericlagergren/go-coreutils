/*
	Go touch - prints the current working directory.
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
	"fmt"
	"log"
	"os"
	"time"

	flag "github.com/ogier/pflag"
)

const (
	Help = `
Usage: touch [OPTION]... FILE...
Update the access and modification times of each FILE to the current time.

A FILE argument that does not exist is created empty, unless -c or -h
is supplied.

A FILE argument string of - is handled specially and causes touch to
change the times of the file associated with standard output.

Mandatory arguments to long options are mandatory for short options too.
  -a                     change only the access time
  -c, --no-create        do not create any files
  -d, --date=STRING      parse STRING and use it instead of current time
  -f                     (ignored)
  -h, --no-dereference   affect each symbolic link instead of any referenced
                         file (useful only on systems that can change the
                         timestamps of a symlink)
  -m                     change only the modification time
  -r, --reference=FILE   use this file's times instead of current time
  -t STAMP               use [[CC]YY]MMDDhhmm[.ss] instead of current time
      --time=WORD        change the specified time:
                           WORD is access, atime, or use: equivalent to -a
                           WORD is modify or mtime: equivalent to -m
      --help     display this help and exit
      --version  output version information and exit

Note that the -d and -t options accept different time-date formats.

`
	Version = `
touch (Go coreutils) 0.1
Copyright (C) 2015 Robert Deusser
License GPLv3+: GNU GPL version 3 or later <http://gnu.org/licenses/gpl.html>.
This is free software: you are free to change and redistribute it.
There is NO WARRANTY, to the extent permitted by law.

`
)

var (
	nocreate = flag.BoolP("no-create", "c", false, "")
	version  = flag.BoolP("version", "v", false, "")
)

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

	if len(flag.Args()) > 0 {
		for i := 0; i < len(flag.Args()); i++ {
			filename := flag.Arg(i)
			_, err := os.Stat(filename)
			if err == nil {
				now := time.Now()
				os.Chtimes(filename, now, now)
			} else if !(*nocreate) {
				f, err := os.OpenFile(filename, os.O_WRONLY|os.O_CREATE, 0644)
				f.Close()
				if err != nil {
					log.Fatal(err)
				}
			}
		}
	}
}
