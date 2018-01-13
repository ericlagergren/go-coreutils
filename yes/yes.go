package main

import (
	"bytes"
	"flag"
	"fmt"
	"os"
)

var version bool

func main() {
	flag.BoolVar(&version, "version", false, "")
	flag.BoolVar(&version, "v", false, "")

	flag.Usage = func() {
		fmt.Printf(`Usage: yes [STRING]...
  or:  yes OPTION
Repeatedly output a line with all specified STRING(s), or 'y'.

      --help     display this help and exit
      --version  output version information and exit

Report yes bugs to eric@ericlagergren.com
Go coreutils home page: <https://www.github.com/ericlagergren/go-coreutils/>
`)
	}
	flag.Parse()

	if version {
		fmt.Printf(`yes (Go coreutils) 1.1
Copyright (C) 2015-2017 Eric Lagergren.
License GPLv3+: GNU GPL version 3 or later <http://gnu.org/licenses/gpl.html>.
This is free software: you are free to change and redistribute it.
There is NO WARRANTY, to the extent permitted by law.
`)
		return
	}

	var buf []byte
	if flag.NArg() == 0 {
		buf = bytes.Repeat([]byte{'y', '\n'}, 4096)
	} else {
		for _, arg := range flag.Args() {
			buf = append(buf, arg...)
			buf = append(buf, ' ')
		}
		buf = append(buf, '\n')
	}

	for {
		os.Stdout.Write(buf)
	}
}
