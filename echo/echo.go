package main

import (
	"fmt"
	"os"
	"strings"
)

func main() {
	fmt.Print(echo())
}

func echo() string {
	// -n argument ommits the trailing new line
	if len(os.Args) >= 2 && os.Args[1] == "-n" {
		return fmt.Sprint(strings.Join(os.Args[2:], " "))

	}

	// with trailing new line
	return fmt.Sprintln(strings.Join(os.Args[1:], " "))
}
