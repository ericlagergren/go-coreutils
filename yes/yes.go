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

	LineFeed = []byte("\n")
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

	// Fixed []byte array will increase performance by x~100
	arg := make([]byte, 4096)
	copy(arg[:], args[0])
	// Add \n at the end of the of "y" or argument
	arg = append(arg[:], LineFeed...)

	for {
		os.Stdout.Write(arg)
	}

}
