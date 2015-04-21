package main

import (
	"fmt"
	"os"

	flag "github.com/ogier/pflag"
)

const (
	Help = `Usage: yes [STRING]...
  or:  yes OPTION
Repeatedly output a line with all specified STRING(s), or 'y'.

      --help     display this help and exit
      --version  output version information and exit

Report yes bugs to ericscottlagergren@gmail.com
Go coreutils home page: <https://www.github.com/EricLagerg/go-coreutils/>
`
	Version = `yes (Go coreutils) 1.0
Copyright (C) 2015 Eric Lagergren.
License GPLv3+: GNU GPL version 3 or later <http://gnu.org/licenses/gpl.html>.
This is free software: you are free to change and redistribute it.
There is NO WARRANTY, to the extent permitted by law.
`
)

var (
	version = flag.BoolP("version", "v", false, "")

	LineFeed = []byte("\n")
	Space    = []byte(" ")
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

	args := flag.Args()
	if flag.NArg() == 0 {
		args = []string{"y"}
	}

	for {
		for i, arg := range args {
			os.Stdout.WriteString(arg)

			if i == len(args)-1 {
				os.Stdout.Write(LineFeed)
			} else {
				os.Stdout.Write(Space)
			}
		}
	}
}
