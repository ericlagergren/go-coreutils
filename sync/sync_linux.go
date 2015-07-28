package main

import (
	"fmt"
	"os"
	"syscall"

	flag "github.com/ogier/pflag"
)

const (
	Help = `Usage: sync [OPTION]
Force changed blocks to disk, update the super block.

      --help     display this help and exit
      --version  output version information and exit

Report wc bugs to ericscottlagergren@gmail.com
Go coreutils home page: <https://www.github.com/EricLagerg/go-coreutils/>
`
	Version = `sync (coreutils) 1
Copyright (C) 2015 Eric Lagergren.
License GPLv3+: GNU GPL version 3 or later <http://gnu.org/licenses/gpl.html>.
This is free software: you are free to change and redistribute it.
There is NO WARRANTY, to the extent permitted by law.

Written by Eric Lagergren <ericscottlagergren@gmail.com>
`
)

var version = flag.BoolP("version", "v", false, "")

func main() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "%s", Help)
		os.Exit(1)
	}
	flag.Parse()

	// Nothing can fail.
	syscall.Sync()
}
