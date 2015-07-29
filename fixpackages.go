package main

import (
	"os"
)

func main() {
	wd, err := os.Getwd()
	if err != nil {
		panic(err)
	}

	dir, err := os.Open(wd)
	if err != nil {
		panic(err)
	}
	defer dir.Close()

	stats, err := dir.Readdir(-1)
	if err != nil {
		panic(err)
	}

}

func fixPackageName(fi os.FileInfo) {

}
