package main

import "fmt"

import (
	_ "github.com/EricLagerg/go-coreutils/wc"
)

const (
	Help = `Usage: go-coreutils [COMMAND] [PACKAGES]...

  COMMAND: install

  -a, --all          install all utils
      --[NAME]       install specific command (use full name)
      --from-file=F  install from text file, F
      --from-file0=F install from NUL-terminated string, F

  COMMAND: remove

  -a, --all          remove all utils
      --[NAME]       remove specific command (use full name)
      --from-file=F  remove from text file, F
      --from-file0=F remove from NUL-terminated string, F

With no PACKAGES or F or either are -, read from standard input.

Examples:
  go-coreutils install --all
  go-coreutils install --rm --fmt --yes --wc
  go-coreutils install --from-file=list.txt
  ... -print0 | go-coreutils install --from-file0=-
`
)

func main() {
	fmt.Printf("%s", Help)
}
