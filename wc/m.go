// +build ignore

package main

import (
	"os"

	"github.com/ericlagergren/go-coreutils/coreutils"

	_ "github.com/ericlagergren/go-coreutils/wc"
)

func main() {
	ctx := coreutils.Ctx{
		Stdin:  os.Stdin,
		Stdout: os.Stdout,
		Stderr: os.Stderr,
	}
	coreutils.Run(ctx, "wc", "-l", "cmd.go")
}
