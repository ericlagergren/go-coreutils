/*
	Go tty -- print the name of the terminal connected to standard input

	Copyright (C) 2014 Eric Lagergren

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

/* Written by Eric Lagergren
Inspired by David MacKenzie <djm@gnu.ai.mit.edu>.  */

package main

import (
	"errors"
	"fmt"
	flag "github.com/ogier/pflag"
	"os"
	"path/filepath"
	"syscall"
	"unsafe"
)

const (
	// The single letters are the abbreviations
	// used by the String method's formatting.
	ModeDir        = 1 << (32 - 1 - iota) // d: is a directory
	ModeAppend                            // a: append-only
	ModeExclusive                         // l: exclusive use
	ModeTemporary                         // T: temporary file (not backed up)
	ModeSymlink                           // L: symbolic link
	ModeDevice                            // D: device file
	ModeNamedPipe                         // p: named pipe (FIFO)
	ModeSocket                            // S: Unix domain socket
	ModeSetuid                            // u: setuid
	ModeSetgid                            // g: setgid
	ModeCharDevice                        // c: Unix character device, when ModeDevice is set
	ModeSticky                            // t: sticky

	// Mask for the type bits. For regular files, none will be set.
	ModeType = ModeDir | ModeSymlink | ModeNamedPipe | ModeSocket | ModeDevice

	ModePerm = 0777 // permission bits
)

const (
	VERSION = `tty (Go coreutils) 1.0
Copyright (C) 2014 Free Software Foundation, Inc.
License GPLv3+: GNU GPL version 3 or later <http://gnu.org/licenses/gpl.html>.
This is free software: you are free to change and redistribute it.
There is NO WARRANTY, to the extent permitted by law.

Written by Eric Lagergren
Inspired by David MacKenzie.`
	HELP = `Usage: tty [OPTION]...
Print the file name of the terminal connected to standard input.

  -s, --silent, --quiet   print nothing, only return an exit status
      --help     display this help and exit
      --version  output version information and exit

Report uname bugs to ericscottlagergren@gmail.com
Go coreutils home page: <https://www.github.com/EricLagerg/go-coreutils/>`
	DEV = "/dev"
)

var (
	NotFound   = errors.New("device not found")
	searchDevs = []string{
		"/dev/console",
		"/dev/wscons",
		"/dev/pts/",
		"/dev/vt/",
		"/dev/term/",
		"/dev/zcons/",
	}
)

func fileMode(longMode uint32) uint32 {
	mode := longMode & ModePerm
	switch longMode & syscall.S_IFMT {
	case syscall.S_IFBLK:
		mode |= ModeDevice
	case syscall.S_IFCHR:
		mode |= ModeDevice | ModeCharDevice
	case syscall.S_IFDIR:
		mode |= ModeDir
	case syscall.S_IFIFO:
		mode |= ModeNamedPipe
	case syscall.S_IFLNK:
		mode |= ModeSymlink
	case syscall.S_IFREG:
		// nothing to do
	case syscall.S_IFSOCK:
		mode |= ModeSocket
	}
	if longMode&syscall.S_ISGID != 0 {
		mode |= ModeSetgid
	}
	if longMode&syscall.S_ISUID != 0 {
		mode |= ModeSetuid
	}
	if longMode&syscall.S_ISVTX != 0 {
		mode |= ModeSticky
	}
	return mode
}

func ttyNameCheckDir(stat syscall.Stat_t, dir string) (string, error) {
	var (
		rs       string
		fullPath string
	)

	fp := true
	if dir == DEV {
		fp = false
	}

	fi, err := os.Open(dir)
	if err != nil {
		return "", err
	}

	names, err := fi.Readdirnames(-1)
	devStat := syscall.Stat_t{}

	for _, name := range names {
		if !fp {
			fullPath = filepath.Join(DEV, name)
		} else {
			fullPath = filepath.Join(filepath.Dir(dir), name)
		}
		err = syscall.Stat(fullPath, &devStat)
		if err != nil {
			continue
		}

		// Directories to skip
		if fullPath == "/dev/stderr" || fullPath == "/dev/stdin" || fullPath == "/dev/stdout" || len(fullPath) >= 8 && fullPath[0:8] == "/dev/fd/" {
			continue
		}

		mode := fileMode(devStat.Mode)
		if mode&ModeDir != 0 {
			rs, err = ttyNameCheckDir(stat, fullPath)
			if err != nil {
				continue
			} else {
				return rs, nil
			}
		} else if mode&ModeCharDevice != 0 && devStat.Ino == stat.Ino && devStat.Rdev == stat.Rdev {
			return fullPath, nil
		}
	}
	return "", NotFound
}

func Isatty(fd uintptr) bool {
	var termios syscall.Termios

	_, _, err := syscall.Syscall6(syscall.SYS_IOCTL, fd,
		uintptr(syscall.TCGETS),
		uintptr(unsafe.Pointer(&termios)),
		0,
		0,
		0)
	return err == 0
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

	var (
		name string
		err  error
	)

	si := os.Stdin.Fd()
	// Preliminary tty check to save time checking directories
	if Isatty(si) {
		good := false

		stat := syscall.Stat_t{}
		_ = syscall.Fstat(int(si), &stat)

		for _, d := range searchDevs {
			name, err = ttyNameCheckDir(stat, d)
			if err == nil {
				good = true
				break
			}
		}

		if !good {
			name, err = ttyNameCheckDir(stat, DEV)
		}

		if !silent {
			if err != nil {
				fmt.Println("tty")
			} else if len(name) > 0 {
				fmt.Println(name)
			}
		} else {
			return
		}
	} else {
		if !silent {
			fmt.Println("not a tty")
		}
		os.Exit(1)
	}
}
