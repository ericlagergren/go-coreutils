package main

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"path"
	"strings"
	"syscall"

	flag "github.com/ogier/pflag"
)

const (
	HELP    = `HELP`
	VERSION = `VERSION`
)

// enum for symlinks (deref)
const (
	derefUndefined = iota
	derefNever
	derefArgs
	derefAlways
)

// enum reflinks
const (
	reflinkNever = iota
	reflinkAuto
	reflinkAlways
)

// enum for interactive
const (
	alwaysYes = iota
	alwaysNo
	alwaysAsk
	unspecified
)

// enum for sparse files
const (
	sparseUnused = iota
	sparseNever
	sparseAuto
	sparseAlways
)

// backup enum
const (
	noBackups = iota
	simpleBackups
	numberedExistingBackups
	numberedBackups
)

var (
	archive           = flag.BoolP("archive", "a", false, "")
	attrOnly          = flag.Bool("attributes-only", false, "")
	backup            = flag.String("backup", "", "")
	backup2           = flag.Bool("b", false, "")
	copyContents      = flag.Bool("copy-contents", false, "")
	ndrpl             = flag.Bool("d", false, "")
	dereference       = flag.BoolP("dereference", "L", false, "")
	force             = flag.BoolP("force", "f", false, "")
	hopt              = flag.Bool("H", false, "")
	interactive       = flag.BoolP("interactive", "i", false, "")
	link              = flag.BoolP("link", "l", false, "")
	noClobber         = flag.BoolP("no-clobber", "n", false, "")
	noDereference     = flag.BoolP("no-dereference", "P", false, "")
	noPreserve        = flag.String("no-preserve", "", "")
	noTargetDir       = flag.BoolP("no-target-directory", "T", false, "")
	oneFS             = flag.BoolP("one-file-system", "x", false, "")
	parents           = flag.Bool("parents", false, "")
	path              = flag.Bool("path", false, "")
	pmot              = flag.Bool("p", false, "")
	preserve          = flag.String("preserve", "", "")
	recursive         = flag.BoolP("recursive", "R", false, "")
	recursive2        = flag.Bool("r", false, "")
	removeDestination = flag.Bool("remove-destination", false, "")
	sparse            = flag.String("sparse", "界", "")
	reflink           = flag.String("reflink", "世", "")
	selinux           = flag.Bool("Z", false, "")
	stripTrailSlash   = flag.Bool("strip-trailing-slashes", false, "")
	suffix            = flag.StringP("suffix", "S", "", "")
	symLink           = flag.BoolP("symbolic-link", "s", false, "")
	targetDir         = flag.StringP("target-directory", "t", "", "")
	update            = flag.BoolP("update", "u", false, "")
	verbose           = flag.BoolP("verbose", "v", false, "")
	version           = flag.Bool("version", false, "")
)

var (
	makeBackups      bool
	copyConts        bool
	parentsOpt       bool
	removeTrailSlash bool
	noTargDir        bool
	targDir          string
	versControl      string
	suffixString     string
)

type Options struct {
	AsRegular         bool
	Dereference       int
	UnlinkBefore      bool
	UnlinkAfterFailed bool
	HardLink          bool
	Interactive       int
	MoveMode          bool
	OneFS             bool
	RefLinkMode       int

	PreserveOwnership      bool
	PreserveLinks          bool
	PreserveMode           bool
	PreserveTimestamps     bool
	ExplicitNoPreserve     bool
	PreserveSecurityContex bool // -a or --preserve=context
	RequirePreserveContext bool // --preserve=contex
	SetSecurityContext     bool // -Z, set sys default contex
	PreserveXattr          bool
	ReduceDiagnostics      bool
	RequirePreserveXattr   bool

	DataCopyRequired bool
	RequirePreserve  bool
	Recursive        bool
	SparseMode       int
	SymbolicLink     bool
	SetMode          bool
	Mode             int
	BackupOpts       int

	Update  bool
	Verbose bool
}

// sparse argument list
var sparseArgList = []string{
	"never",
	"auto",
	"always",
}

// reflink argument list
var reflinkArgList = []string{
	"auto",
	"always",
}

// Find length of a C-style string
func clen(n string) int {
	for i := 0; i < len(n); i++ {
		if n[i] == 0 {
			return i
		}
	}
	return len(n)
}

func stripSlash(str string) string {
	if len(str) > 0 && os.IsPathSeparator(str[len(str)-1]) {
		return str[0 : len(str)-1]
	}
	return str
}

// returns true if `file` is a directory as well as a pointer to
// a os.FileInfo struct
func isDir(file string) (bool, *os.FileInfo) {
	info, err := os.Stat(file)
	if err != nil {
		if err.(*os.PathError).Err != syscall.ENOENT {
			log.Fatalf("%s failed to access %s", err, file)
		}
		return false, nil
	}
	return info.Mode().IsDir(), &info
}

func getVersion(version string) int {

	argList := []string{
		"none", "off", // 0
		"simple", "never", // 1
		"existing", "nil", // 2
		"numbered", "t", // 3
	}

	if version != "" {
		return argmatch(version, argList)
	} else {
		if version == "" {
			return numberedExistingBackups
		}
		return argmatch(os.Getenv("VERSION_CONTROL"), argList)
	}

}

// check to see if the given context argument is valid
// returns integer >= 0 (index of list) if true
// returns -1 if no match found
func argmatch(arg string, list []string) int {
	for i, v := range list {
		if arg == v ||
			// a more in-depth test for edge cases
			bytes.Equal([]byte(arg[:clen(arg)]),
				[]byte(v[:clen(v)])) {
			return i
		}
	}
	return -1
}

// set struct Options members' values depending on given string, `args`
func (o *Options) decodePreserve(args *string, dep bool) {
	// file attr enum
	const (
		mode = iota
		time
		owner
		link
		context
		xattr
		all
	)

	argList := []string{
		"mode",
		"timestamps",
		"ownership",
		"links",
		"context",
		"xattr",
		"all",
	}

	l := strings.Split(*args, ",")
	for _, v := range l {
		switch argmatch(v, argList) {
		case -1:
			log.Printf("invalid argument %s\n", v)
		case mode:
			o.PreserveMode = dep
			o.ExplicitNoPreserve = !dep
		case time:
			o.PreserveTimestamps = dep
		case owner:
			o.PreserveOwnership = dep
		case link:
			o.PreserveLinks = dep
		case context:
			o.RequirePreserveContext = dep
			o.PreserveSecurityContex = dep
		case xattr:
			o.PreserveXattr = dep
			o.RequirePreserveXattr = dep
		case all:
			o.PreserveMode = dep
			o.PreserveTimestamps = dep
			o.PreserveOwnership = dep
			o.PreserveLinks = dep
			o.ExplicitNoPreserve = !dep
			// selinux
			o.PreserveXattr = dep
		}
	}

}

func cp(n int, files []string, dir string, noDir bool, options *Options) bool {
	var dest string

	if n <= 0 {
		log.Fatalln("missing files operand")
	}

	if noDir {
		if dir != "" {
			log.Fatalln("cannot combine --target-directory (-t) and --no-target-directory (-T)")
		}

		if 2 < n {
			log.Fatalf("extra operand %s\n", files[2])
		}
	} else if dir == "" {
		if 2 <= n {
			if ok, _ := isDir(files[n-1]); ok {
				dir = files[n-1]
			}
		} else if 2 < n {
			log.Fatalf("target %s is not a directory", files[n-1])
		}
	}

	if dir != "" {
		for _, v := range files {
			if removeTrailSlash {
				v = stripSlash(v)
			}

			if parentsOpt {
				dest = path.Join(targDir, v)
			}
		}
	}
	return true
}

func main() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "%s\n", HELP)
		return
	}
	flag.Parse()

	if *version {
		fmt.Printf("%s\n", VERSION)
		return
	}

	o := &Options{
		AsRegular:   true,
		Dereference: derefUndefined,
	}

	if *sparse != "界" {
		if v := argmatch(*sparse, sparseArgList); v >= 0 {
			o.SparseMode = v
		} else {
			log.Fatalf("invalid agument %s\n", *sparse)
		}
	}

	if *reflink != "世" {
		if *reflink == "" {
			o.RefLinkMode = reflinkAlways
		} else {
			if v := argmatch(*reflink, reflinkArgList); v >= 0 {
				o.RefLinkMode = v
			} else {
				log.Fatalf("invalid agument %s\n", *reflink)
			}
		}
	}

	if *archive {
		o.Dereference = derefNever
		o.PreserveLinks = true
		o.PreserveOwnership = true
		o.PreserveMode = true
		o.PreserveTimestamps = true
		o.RequirePreserve = true
		//if selinux is enabled o.PreserveSecurity = true
		log.Println("unable to preserve security context at the moment")
		o.PreserveXattr = true
		o.ReduceDiagnostics = true
		o.Recursive = true
	}

	if *backup != "" || *backup2 {
		makeBackups = true
		if *backup != "" {
			versControl = *backup
		}
	}

	if *attrOnly {
		o.DataCopyRequired = false
	}

	if *copyContents {
		copyConts = true
	}

	if *ndrpl {
		o.PreserveLinks = true
		o.Dereference = derefNever
	}

	if *force {
		o.UnlinkAfterFailed = true
	}

	if *hopt {
		o.Dereference = derefArgs
	}

	if *interactive {
		o.Interactive = alwaysAsk
	}

	if *link {
		o.HardLink = true
	}

	if *dereference {
		o.Dereference = derefAlways
	}

	if *noClobber {
		o.Interactive = alwaysNo
	}

	if *noDereference {
		o.Dereference = derefNever
	}

	if *noPreserve != "" {
		o.decodePreserve(noPreserve, false)
	}

	if *preserve == "" && *pmot {
		o.PreserveOwnership = true
		o.PreserveMode = true
		o.PreserveTimestamps = true
		o.RequirePreserve = true
	} else if *preserve != "" {
		o.decodePreserve(preserve, true)
	}

	if *pmot {
		o.PreserveOwnership = true
		o.PreserveMode = true
		o.PreserveTimestamps = true
		o.RequirePreserve = true
	}

	if *parents {
		parentsOpt = true
	}

	if *recursive || *recursive2 {
		o.Recursive = true
	}

	if *removeDestination {
		o.UnlinkBefore = true
	}

	if *stripTrailSlash {
		removeTrailSlash = true
	}

	if *symLink {
		o.SymbolicLink = true
	}

	if *targetDir != "" {
		if s, err := os.Stat(*targetDir); err != nil {
			log.Fatalf("failed to acces %s\n", *targetDir)
		} else {
			if !s.Mode().IsDir() {
				log.Fatalf("target %s is not a directory\n", *targetDir)
			}
			targDir = *targetDir
		}
	}

	if *noTargetDir {
		noTargDir = true
	}

	if *update {
		o.Update = true
	}

	if *verbose {
		o.Verbose = true
	}

	if *oneFS {
		o.OneFS = true
	}

	if *selinux {
		log.Fatalln("no current way to detect SELinux yet, sorry")
	}

	if *suffix != "" {
		makeBackups = true
		suffixString = *suffix
	}

	if o.HardLink && o.SymbolicLink {
		log.Fatal("cannot make both hard and symbolic links")
	}

	if makeBackups && o.Interactive == alwaysNo {
		log.Fatalln("options --backup and --no-clobber are mutually exclusive")
	}

	if o.RefLinkMode == reflinkAlways && o.SparseMode != sparseAuto {
		log.Fatalln("--reflink can only be used with --sparse=auto")
	}

	if suffixString != "" {
		if makeBackups {
			o.BackupOpts = getVersion(versControl)
		} else {
			o.BackupOpts = noBackups
		}
	}

	if o.Dereference == derefUndefined {
		if o.Recursive && !o.HardLink {
			o.Dereference = derefNever
		} else {
			o.Dereference = derefAlways
		}
	}

	if o.Recursive {
		o.AsRegular = copyConts
	}

	if o.UnlinkAfterFailed && (o.HardLink || o.SymbolicLink) {
		o.UnlinkBefore = true
	}

	if cp(flag.NArg()-1, flag.Args(), targDir, noTargDir, o) {
		return
	}
	os.Exit(1)
}
