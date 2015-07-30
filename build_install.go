package main

import "os"

var install = &Command{
	Name:      "install",
	Run:       installUtils,
	UsageLine: "install [-a, --all] [--files-from{0}] [utilities]",
	Short:     "install all or some utilities",
	Long: `Install will install all or specific packages.
By default the utilities are given the same name as their GNU counterparts,
and are installed in %s ($GOBIN).

If -a or --all is specified, *all* the utilitied will be installed, regardless
of any other utilitied listed. If --files-from is specified, the utility names
will be read from the specified file, which is assumed to be a plain text file
with each command on a new line. If --files-from0 is specified, the utilities
are assumed to be a null-delimited string, much like the output of find's
-print0. Both of the --files-from will cause the program to ignore any other
arguments.

The install flags -- shared with overwrite and remove -- are as follows:
	
	-a, --all           install all utilities
	-d, --dir           install utilities in specific dir (default %s ($GOBIN))
		--files-from=F  install files from F (newline delimited)
		--files-from0=F install files from F (null-delimited)

`,
}

var gobin = os.Getenv("GOBIN")

func installUtils(names []string) {
	for _, name := range names {
		if _, ok := utilities[name]; ok {

		}
	}
}
