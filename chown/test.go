package main

import (
	"fmt"
	"os"
	"path/filepath"
)

func main() {
	/*fi, err := os.Stat("/home/eric/github-repos/go-coreutils/.git")
	if err != nil {
		panic(err)
	}*/
	dir, err := os.Open("/home/eric/github-repos/go-coreutils/")
	if err != nil {
		panic(err)
	}
	//fmt.Printf("%+v\n", fi)
	names, err := dir.Readdirnames(-1)
	for _, name := range names {
		relName, err := filepath.Rel("go-coreutils/", name)
		if err != nil {
			fmt.Println(err)
		}
		fmt.Printf("%s\n", relName)
	}
}
