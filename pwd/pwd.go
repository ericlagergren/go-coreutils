/*
	Go pwd - prints the current working directory.
	Copyright (C) 2015 Robert Deusser
	This program is free software: you can redistribute it and/or modify
	it under the terms of the GNU General Public License as published by
	the Free Software Foundation, either version 3 of the License, or
	(at your option) any later version.
	This program is distributed in the hope that it will be useful,
	but WITHOUT ANY WARRANTY; without even the implied warranty of
	MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
	GNU General Public License for more details.
	You should have received a copy of the GNU General Public License
	along with this program.  If not, see <http://www.gnu.org/licenses/>.
*/

/*
	Written by Robert Deusser <iamthemuffinman@outlook.com>
*/

package main

import (
	"fmt"
	"os"
	"runtime"
	"sync"
	"syscall"

	flag "github.com/ogier/pflag"
)

const (
	Help = `
NAME
       pwd - print name of current/working directory

SYNOPSIS
       pwd [OPTION]...

DESCRIPTION
       Print the full filename of the current working directory.

       -L, --logical
              use PWD from environment, even if it contains symlinks

       -P, --physical
              avoid all symlinks

       --help display this help and exit

       --version
              output version information and exit

       If no option is specified, -P is assumed.

AUTHOR
       Written by Robert Deusser

REPORTING BUGS
       Report wc bugs to ericscottlagergren@gmail.com
       Go coreutils home page: <https://www.github.com/EricLagergren/go-coreutils/>

COPYRIGHT
       Go pwd - Print the full filename of the current working directory.
       Copyright (C) 2015 Robert Deusser

       This program is free software: you can redistribute it and/or modify
       it under the terms of the GNU General Public License as published by
       the Free Software Foundation, either version 3 of the License, or
       (at your option) any later version.

       This program is distributed in the hope that it will be useful,
       but WITHOUT ANY WARRANTY; without even the implied warranty of
       MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
       GNU General Public License for more details.

       You should have received a copy of the GNU General Public License
       along with this program.  If not, see <http://www.gnu.org/licenses/>.

`
	Version = `
pwd (Go coreutils) 0.1
Copyright (C) 2015 Robert Deusser
License GPLv3+: GNU GPL version 3 or later <http://gnu.org/licenses/gpl.html>.
This is free software: you are free to change and redistribute it.
There is NO WARRANTY, to the extent permitted by law.

`
)

var (
	logical  = flag.BoolP("logical", "L", false, "")
	physical = flag.BoolP("physical", "P", false, "")
	version  = flag.BoolP("version", "V", false, "")
)

// From here till main is the Getwd function from the os package rewritten to NOT follow symlinks

var getwdCache struct {
	sync.Mutex
	dir string
}

// useSyscallwd determines whether to use the return value of
// syscall.Getwd based on its error.
var useSyscallwd = func(error) bool { return true }

// Getwd returns a rooted path name corresponding to the
// current directory.  If the current directory can be
// reached via multiple paths (due to symbolic links),
// Getwd may return any one of them.
func GetwdWithoutSymlinks() (dir string, err error) {
	if runtime.GOOS == "windows" {
		return syscall.Getwd()
	}

	// Clumsy but widespread kludge:
	// if $PWD is set and matches ".", use it.
	dot, err := os.Lstat(".")
	if err != nil {
		return "", err
	}
	dir = os.Getenv("PWD")
	if len(dir) > 0 && dir[0] == '/' {
		d, err := os.Lstat(dir)
		if err == nil && os.SameFile(dot, d) {
			return dir, nil
		}
	}

	// If the operating system provides a Getwd call, use it.
	// Otherwise, we're trying to find our way back to ".".
	if syscall.ImplementsGetwd {
		s, e := syscall.Getwd()
		if useSyscallwd(e) {
			return s, os.NewSyscallError("getwd", e)
		}
	}

	// Apply same kludge but to cached dir instead of $PWD.
	getwdCache.Lock()
	dir = getwdCache.dir
	getwdCache.Unlock()
	if len(dir) > 0 {
		d, err := os.Lstat(dir)
		if err == nil && os.SameFile(dot, d) {
			return dir, nil
		}
	}

	// Root is a special case because it has no parent
	// and ends in a slash.
	root, err := os.Lstat("/")
	if err != nil {
		// Can't stat root - no hope.
		return "", err
	}
	if os.SameFile(root, dot) {
		return "/", nil
	}

	// General algorithm: find name in parent
	// and then find name of parent.  Each iteration
	// adds /name to the beginning of dir.
	dir = ""
	for parent := ".."; ; parent = "../" + parent {
		if len(parent) >= 1024 { // Sanity check
			return "", syscall.ENAMETOOLONG
		}
		fd, err := os.Open(parent)
		if err != nil {
			return "", err
		}

		for {
			names, err := fd.Readdirnames(100)
			if err != nil {
				fd.Close()
				return "", err
			}
			for _, name := range names {
				d, _ := os.Lstat(parent + "/" + name)
				if os.SameFile(d, dot) {
					dir = "/" + name + dir
					goto Found
				}
			}
		}

	Found:
		pd, err := fd.Stat()
		if err != nil {
			return "", err
		}
		fd.Close()
		if os.SameFile(pd, root) {
			break
		}
		// Set up for next round.
		dot = pd
	}

	// Save answer as hint to avoid the expensive path next time.
	getwdCache.Lock()
	getwdCache.dir = dir
	getwdCache.Unlock()

	return dir, nil
}

func main() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "%s", Help)
		os.Exit(1)
	}
	flag.Parse()

	switch {
	case *version:
		fmt.Fprintf(os.Stdout, "%s", Version)
		os.Exit(0)
	case *logical:
		dir, err := os.Getwd()
		if err != nil {
			os.Exit(1)
		}
		fmt.Println(dir)
	case *physical:
		dir, err := GetwdWithoutSymlinks()
		if err != nil {
			os.Exit(1)
		}
		fmt.Println(dir)
	default:
		dir, err := GetwdWithoutSymlinks()
		if err != nil {
			os.Exit(1)
		}
		fmt.Println(dir)
	}
}
