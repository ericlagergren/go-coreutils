package main

var remove = &Command{
	Name:      "remove",
	Run:       removeUtils,
	UsageLine: "remove [-a, --all] [--files-from{0}] [utilities]",
	Short:     "remove all or some utilities",
	Long: `Remove will remove all or specific packages.
By default the utilities are given the same name as their GNU counterparts,
and are removed in %s ($GOBIN).

If -a or --all is specified, *all* the utilities will be removed, regardless
of any other utilitied listed. If --files-from is specified, the utility names
will be read from the specified file, which is assumed to be a plain text file
with each command on a new line. If --files-from0 is specified, the utilities
are assumed to be a null-delimited string, much like the output of find's
-print0. Both of the --files-from will cause the program to ignore any other
arguments.

The remove flags -- shared with overwrite and remove -- are as follows:
	
	-a, --all           remove all utilities
	-d, --dir           remove utilities in specific dir (default %s ($GOBIN))
		--files-from=F  remove files from F (newline delimited)
		--files-from0=F remove files from F (null-delimited)

`,
}

func removeUtils(names []string) {
	for _, name := range names {
		if _, ok := utilities[name]; ok {

		}
	}
}
