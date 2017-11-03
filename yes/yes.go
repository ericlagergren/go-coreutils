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
Go coreutils home page: <https://www.github.com/EricLagergren/go-coreutils/>
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

	lineFeed = "\n"
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
	// Add \n at the end of the of "y" or argument
	args[0] = args[0] + lineFeed

	// Fixed []byte array will increase performance by x~100
	arg := make([]byte, 4096)

	for n := 0; n < (len(arg) + 1 - len(args[0])); n += len(args[0]) {
		copy(arg[n:], args[0])
	}

	for {
		os.Stdout.Write(arg)
	}

}
