package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
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

var (
	version = flag.BoolP("version", "v", false, "")
	fatal   = log.New(os.Stderr, "", 0)
)

func main() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "%s", Help)
		os.Exit(1)
	}
	flag.Parse()

	// "To flush all open files on a volume, call FlushFileBuffers with a handle to the volume.
	// The caller must have administrative privileges. For more information, see Running with Special Privileges."
	// https://msdn.microsoft.com/en-us/library/windows/desktop/aa364439(v=vs.85).aspx
	fp := filepath.VolumeName(os.Getwd())
	file, err := os.Open(fp)
	if err != nil {
		fatal.Fatalln(err)
	}

	syscall.Fsync(int(file.Fd()))
}
