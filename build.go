package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"text/template"

	flag "github.com/EricLagergren/pflag"
)

var (
	usageTemplate = `go-coreutils helps you manage Go-coreutils.

Usage:

	go-coreutils COMMAND [ARGUMENTS]...

The commands are:
{{range .}}
	{{.Name | printf "%-11s"}} {{.Short}}{{end}}

Use "go-coreutils help [COMMAND]" for more information about a command.

`

	helpTemplate = `usage: go-coreutils {{.UsageLine}}

{{.Long | trim}}
`
)

var gobin = os.Getenv("GOBIN")

// Command implements a specific command, e.g., go-coreutils install
type Command struct {
	Run func(names []string)

	// Name of the command, e.g., install, overwrite...
	Name string

	// Usage is the usage message, e.g. "name [ARGUMENTS]..."
	UsageLine string

	// Short command tl;dr.
	Short string

	// go-coreutils help <command> output.
	Long string

	// Flags for this command.
	Flag flag.FlagSet
}

func (c *Command) Usage() {
	fmt.Fprintf(os.Stderr, "Usage: %s\n\n", c.UsageLine)
	fmt.Fprintf(os.Stderr, "%s\n", strings.TrimSpace(c.Long))
	os.Exit(1)
}

var commands = [...]*Command{
	install,
	remove,
}

func executeTemplate(w io.Writer, tmpl string, data interface{}) {
	t := template.New("top")
	template.Must(t.Parse(tmpl))
	if err := t.Execute(w, data); err != nil {
		panic(err)
	}
}

func usage(w io.Writer) {
	executeTemplate(w, usageTemplate, commands)
}

func fusage() {
	usage(os.Stderr)
}

func help(args []string) {
	if len(args) == 0 {
		usage(os.Stdout)
		return
	}

	if len(args) != 1 {
		fmt.Fprintf(os.Stderr, "usage: go-coreutils help command\n\nToo many arguments given")
		os.Exit(1)
	}

	arg := args[0]

	for _, cmd := range commands {
		if cmd.Name == arg {
			executeTemplate(os.Stdout, helpTemplate, cmd)
			return
		}
	}

	fmt.Fprintf(os.Stderr, "Unknown help topic %#q. Run 'go-coreutils help'\n", arg)
	os.Exit(1)
}

var (
	once      sync.Once
	empty     struct{}
	utilities map[string]struct{}
)

func buildUtilMap() {
	utilities = make(map[string]struct{})

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

	for _, info := range stats {
		name := filepath.Base(info.Name())
		if name != "Godeps" &&
			info.Mode().IsDir() &&
			strings.Index(name, ".") == -1 {

			utilities[name] = empty
		}
	}
}

func loopUtilities(names []string) {
	if *all {
		for _, name := range names {
			doAction(name)
		}
	} else {
		for _, name := range names {
			if _, ok := utilities[name]; ok {
				doAction(name)
			}
		}
	}
}

func doAction(name string) {
	if complicated {
		doComplicated(name)
	} else {
		doSimple(name)
	}
}

func main() {
	flag.Usage = fusage
	flag.Parse()

	args := flag.Args()
	if len(args) < 1 {
		usage(os.Stderr)
		os.Exit(1)
	}

	if args[0] == "help" {
		help(args)
	}

	once.Do(buildUtilMap)

	for _, cmd := range commands {
		if cmd.Name == args[0] {
			cmd.Flag.Usage = func() { cmd.Usage() }
			cmd.Flag.Parse(args[1:])
			args = cmd.Flag.Args()
			cmd.Run(args)
			return
		}
	}

	fmt.Fprintf(os.Stderr, "go-coreutils: unknown command %q\nRun 'go-coreutils help for usage'\n", args[0])
}
