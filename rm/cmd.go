package rm

import (
	"errors"
	"fmt"
	"io"

	coreutils "github.com/ericlagergren/go-coreutils"
	flag "github.com/spf13/pflag"
)

func init() {
	coreutils.Register("rm", run)
}

// Sentinal flags for default values or flags with single-character options and
// without multi-character options. (e.g., if we want -i but not --i.)
const (
	uniNonChar   = 0xFDD0
	interDefault = string(uniNonChar + 1)
	bad1         = string(uniNonChar + 2)
	bad2         = string(uniNonChar + 3)
	bad3         = string(uniNonChar + 4)
)

func newCommand() *cmd {
	var c cmd
	c.f.BoolVarP(&c.force, "force", "f", false, "ignore non-existent files and arguments; never prompt prior to removal")
	c.f.BoolVarP(&c.moreInter, bad1, "i", false, "prompt before each removal")
	c.f.BoolVarP(&c.lessInter, bad2, "I", false, "prompt (once) prior to removing more than three files or when removing recursively")
	c.f.StringVar(&c.interactive, "interactive", interDefault, "prompt: 'never', 'once' (-i), 'always' (-I)")
	c.f.BoolVar(&c.oneFileSystem, "one-file-system", false, "when recursing, skip directories that are on a different filesystem")
	c.f.BoolVar(&c.preserveRoot, "preserve-root", true, "do not remove '/'")
	c.f.BoolVar(&c.noPreserveRoot, "no-preserve-root", false, "do not special-case '/'")
	c.f.BoolVarP(&c.recursive, "recursive", "r", false, "remove directories and their contents recursively")
	c.f.BoolVarP(&c.recursive, bad3, "R", false, "remove directories and their contents recursively")
	c.f.BoolVarP(&c.rmdir, "dir", "d", false, "remove empty directories")
	c.f.BoolVarP(&c.verbose, "verbose", "v", false, "explain what's occurring")
	c.f.BoolVar(&c.version, "version", false, "print version information and exit")
	return &c
}

type cmd struct {
	f                    flag.FlagSet
	force                bool
	moreInter, lessInter bool
	interactive          string
	preserveRoot         bool
	noPreserveRoot       bool
	oneFileSystem        bool
	recursive            bool
	rmdir                bool
	verbose              bool
	version              bool
}

func run(ctx coreutils.Context, args ...string) error {
	c := newCommand()
	if err := c.f.Parse(args); err != nil {
		return err
	}

	if c.version {
		fmt.Fprintf(ctx.Stdout, "rm (go-coreutils) 1.0")
		return nil
	}

	var opts RemoveOption
	if c.noPreserveRoot && !c.preserveRoot {
		opts |= NoPreserveRoot
	}
	if c.force {
		opts |= Force
	}
	if c.recursive {
		opts |= Recursive
	}
	if c.rmdir {
		opts |= RemoveEmpty
	}
	if c.oneFileSystem {
		opts |= OneFileSystem
	}
	if c.verbose {
		opts |= Verbose
	}
	switch c.interactive {
	case interDefault:
		if c.moreInter {
			opts |= PromptAlways
			c.lessInter = false
		}
	case "never", "no", "none":
		opts &= PromptAlways
	case "once":
		c.lessInter = true
		opts &= IgnoreMissing
	case "always", "yes", "":
		opts |= PromptAlways
		opts &= IgnoreMissing
	default:
		return errors.New("unknown interactive option: " + c.interactive)
	}

	if c.lessInter && (opts&Recursive != 0 || c.f.NArg() >= 3) {
		n := c.f.NArg()
		arg := "arguments"
		adj := ""
		if opts&Recursive != 0 {
			adj = " recursively "
			if n == 1 {
				arg = "argument"
			}
		}
		fmt.Fprintf(ctx.Stderr, "rm: remove %d %s%s? ", n, arg, adj)
		switch yes, err := getYesNo(ctx.Stdin); {
		case err != nil:
			return err
		case !yes:
			return nil
		}
	}

	r := NewRemover(opts)

	if r.opts&PromptAlways != 0 {
		r.Prompt = func(name string, opts PromptOption) bool {

			wp := " "
			if opts&WriteProtected != 0 {
				wp = " write-protected "
			}

			msg := "rm: remove%s%s %q? "
			typ := "file"
			if opts&(Descend|Directory) != 0 {
				typ = "directory"
				if opts&Descend != 0 {
					msg = "rm: descend into%s%s %q? "
				}
			}

			fmt.Fprintf(ctx.Stderr, msg, wp, typ, name)
			yes, err := getYesNo(ctx.Stdin)
			return yes && err == nil
		}
	}

	if r.Log != nil {
		defer close(r.Log)
		go func() {
			for msg := range r.Log {
				fmt.Fprintln(ctx.Stdout, msg)
			}
		}()
	}

	var nerrs int
	for _, name := range c.f.Args() {
		switch err := r.Remove(name); err.(type) {
		case nil:
			// OK
		case rmError:
			fmt.Fprintf(ctx.Stderr, "rm: %v\n", err)
		default:
			fmt.Fprintf(ctx.Stderr, "rm: %v\n", err)
			return err
		}
	}
	if nerrs > 0 {
		return errNonFatal
	}
	return nil
}

var errNonFatal = errors.New("at least one non-fatal error occurred")

func getYesNo(r io.Reader) (yes bool, err error) {
	var resp string
	fmt.Fscanln(r, &resp)
	switch resp {
	case "yes", "Y", "y":
		return true, nil
	case "no", "N", "n":
		return false, nil
	default:
		return false, errors.New("unknown response (must be 'yes' or 'no')")
	}
}
