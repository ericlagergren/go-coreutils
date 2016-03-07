package main

import (
	"fmt"
	"log"
	"os"
	"sort"

	"github.com/EricLagergren/go-coreutils/internal/flag"

	"github.com/EricLagergren/go-gnulib/utmp"
)

func main() {
	flag.Usage = func() {
		fmt.Printf("Usage: %s [OPTION]... [FILE]\n", flag.Program)
		fmt.Printf(`Output who is currently logged in according to FILE.
If FILE is not specified, use %s. %s as FILE is common.

`,
			utmp.UtmpxFile, utmp.Wtmpxfile)
		flag.PrintDefaults()
		fmt.Printf(`
Report %s bugs to ericscottlagergren@gmail.com
Go coreutils home page: <https://www.github.com/EricLagergren/go-coreutils/>
`, flag.Program)
		os.Exit(0)
	}

	flag.Parse()

	switch flag.NArg() {
	case 0:
		users(utmp.UtmpxFile, utmp.CheckPIDs)
	case 1:
		users(flag.Arg(0), 0)
	default:
		log.Fatalf("extra operand(s) %v", flag.Args()[1:])
	}

}

func users(file string, opts int) {
	utmps, err := utmp.ReadUtmp(utmp.UtmpxFile, utmp.CheckPIDs)
	if err != nil {
		log.Fatalln(err)
	}

	var names []string
	for _, u := range utmps {
		if u.IsUserProcess() {
			names = append(names, u.ExtractTrimmedName())
		}
	}
	sort.Strings(names)
	for _, name := range names {
		fmt.Println(name)
	}
}
