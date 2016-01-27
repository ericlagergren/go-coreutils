package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	flag "github.com/EricLagergren/pflag"
)

var install = &Command{
	Name:      "install",
	Run:       installUtils,
	UsageLine: "install [-a, --all] [--files-from{0}] [utilities]",
	Short:     "install all or some utilities",
	Long: `Install will install all or specific packages.
By default the utilities are given the same name as their GNU counterparts,
and are installed in %s ($GOBIN).

If -a or --all is specified, *all* the utilities will be installed, regardless
of any other utilitied listed. If --files-from is specified, the utility names
will be read from the specified file, which is assumed to be a plain text file
with each command on a new line. If --files-from0 is specified, the utilities
are assumed to be a null-delimited string, much like the output of find's
-print0. Both of the --files-from will cause the program to ignore any other
arguments.

The install flags -- shared with overwrite and remove -- are as follows:
	
	-a, --all           install all utilities
	-d, --dir           install utilities in specific dir (default %s ($GOBIN))
	-o, --overwrite     overwrite current GNU coreutils (if they exist)
	-p, --prefix=S      prefix command with S, e.g., if -p=go-, then
	                    the command file would be named go-wc, go-cat, etc.
		--files-from=F  install files from F (newline delimited)
		--files-from0=F install files from F (null-delimited)

`,
}

var (
	all       = flag.BoolP("all", "a", false, "")
	overwrite = flag.BoolP("overwrite", "o", false, "")
	prefix    = flag.StringP("prefix", "p", "", "")
	dir       = flag.StringP("dir", "d", "", "")

	complicated = *overwrite || *prefix != "" || *dir != ""
)

func installUtils(names []string) {
	if gobin == "" && *dir == "" {
		fmt.Println("go-coreutils: Cannot have empty $GOBIN and empty --dir")
		os.Exit(1)
	}

	if len(names) < 1 {
		fmt.Println("go-coreutils: Must list utilities to install or use --all option")
		os.Exit(1)
	}

	loopUtilities(names)
}

func goGenerate(name string) { runCmd(name, "go", "generate") }

func doSimple(name string) {
	goGenerate(name)
	runCmd(name, "go", "install")
}

func doComplicated(name string) {
	goGenerate(name)
	runCmd(name, "go", "build")

	if *dir == "" {
		*dir = filepath.Dir(runCmdWithOutput("", "which", name))
	}

	newName := *prefix + name
	if !*overwrite {
		name = filepath.Join(*dir, *prefix+name+".2")
	}

	runCmd(name, "mv", name, newName)
}

func runCmd(dir string, args ...interface{}) {
	cmds := list(args...)
	cmd := exec.Command(cmds[0], cmds[1:]...)
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		panic(err)
	}
}

func runCmdWithOutput(dir string, args ...interface{}) string {
	cmds := list(args...)
	cmd := exec.Command(cmds[0], cmds[1:]...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		os.Stderr.Write(out)
		panic(err)
		out = nil
	}
	return string(bytes.TrimSpace(out))
}

func list(args ...interface{}) []string {
	var list []string
	for _, arg := range args {
		switch arg := arg.(type) {
		case []string:
			list = append(list, arg...)
		case string:
			list = append(list, arg)
		default:
			panic("stringList: invalid argument")
		}
	}
	return list
}
