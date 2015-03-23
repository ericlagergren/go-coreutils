/*
	Go chown -- change ownership of a file

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

/*
	Written by Eric Lagergren <ericscottlagergren@gmail.com>
	Inspired by GNU's chown-core (Extracted from chown.c/chgrp.c and librarified by Jim Meyering.)
*/

// BUG(eric): -R flag could get stuck in an infinite loop

package main

import (
	"errors"
	"fmt"
	flag "github.com/ogier/pflag"
	"log"
	"os"
	"os/user"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"syscall"
)

type RCStatus int
type CHStatus int

const (
	HELP = `Usage: chown [OPTION]... [OWNER][:[GROUP]] FILE...
  or:  chown [OPTION]... --reference=RFILE FILE...
Change the owner and/or group of each FILE to OWNER and/or GROUP.
With --reference, change the owner and group of each FILE to those of RFILE.

  -c, --changes          like verbose but report only when a change is made
  -f, --silent, --quiet  suppress most error messages
  -v, --verbose          output a diagnostic for every file processed
      --dereference      affect the referent of each symbolic link (this is
                         the default), rather than the symbolic link itself
  -h, --no-dereference   affect symbolic links instead of any referenced file
                         (useful only on systems that can change the
                         ownership of a symlink)
      --from=CURRENT_OWNER:CURRENT_GROUP
                         change the owner and/or group of each file only if
                         its current owner and/or group match those specified
                         here.  Either may be omitted, in which case a match
                         is not required for the omitted attribute
      --no-preserve-root  do not treat '/' specially (the default)
      --preserve-root    fail to operate recursively on '/'
      --reference=RFILE  use RFILE's owner and group rather than
                         specifying OWNER:GROUP values
  -R, --recursive        operate on files and directories recursively

The following options modify how a hierarchy is traversed when the -R
option is also specified.  If more than one is specified, only the final
one takes effect.

  -H                     if a command line argument is a symbolic link
                         to a directory, traverse it
  -L                     traverse every symbolic link to a directory
                         encountered
  -P                     do not traverse any symbolic links (default)

      --help     display this help and exit
      --version  output version information and exit

Owner is unchanged if missing.  Group is unchanged if missing, but changed
to login group if implied by a ':' following a symbolic OWNER.
OWNER and GROUP may be numeric as well as symbolic.

Examples:
  chown root /u        Change the owner of /u to "root".
  chown root:staff /u  Likewise, but also change its group to "staff".
  chown -hR root /u    Change the owner of /u and subfiles to "root".

Report wc bugs to ericscottlagergren@gmail.com
Go coreutils home page: <https://www.github.com/EricLagerg/go-coreutils/>`
	VERSION = `chown (Go coreutils) 1.0
Copyright (C) 2014 Eric Lagergren
License GPLv3+: GNU GPL version 3 or later <http://gnu.org/licenses/gpl.html>.
This is free software: you are free to change and redistribute it.
There is NO WARRANTY, to the extent permitted by law.

Written by Eric Lagergren.
Inspired by David MacKenzie and Jim Meyering.`

	EXIT_FAILURE = `Try 'chown --help' for more information`
	MAX_INT      = int(^uint(0) >> 1)
)

// Copied from http://golang.org/src/pkg/os/types.go
const (
	// The single letters are the abbreviations
	// used by the String method's formatting.
	ModeDir        uint32 = 1 << (32 - 1 - iota) // d: is a directory
	ModeAppend                                   // a: append-only
	ModeExclusive                                // l: exclusive use
	ModeTemporary                                // T: temporary file (not backed up)
	ModeSymlink                                  // L: symbolic link
	ModeDevice                                   // D: device file
	ModeNamedPipe                                // p: named pipe (FIFO)
	ModeSocket                                   // S: Unix domain socket
	ModeSetuid                                   // u: setuid
	ModeSetgid                                   // g: setgid
	ModeCharDevice                               // c: Unix character device, when ModeDevice is set
	ModeSticky                                   // t: sticky

	// Mask for the type bits. For regular files, none will be set.
	ModeType = ModeDir | ModeSymlink | ModeNamedPipe | ModeSocket | ModeDevice

	ModePerm = 0777 // permission bits
)

const (
	_  = iota // Don't need 0
	__ = iota // Don't need 1

	// fchown succeeded
	RC_OK RCStatus = iota

	// uid/gid are specified and don't match
	RC_EXCLUDED RCStatus = iota

	// SAME_INODE failed
	RC_INODE_CHANGED RCStatus = iota

	// open/fchown isn't needed, safe, or doesn't work so use chown
	RC_DO_ORDINARY_CHOWN RCStatus = iota

	// open, fstat, fchown, or close failed
	RC_ERROR RCStatus = iota
)

const (
	_                               = iota // Don't need 0
	CH_NOT_APPLIED         CHStatus = iota
	CH_SUCCEEDED           CHStatus = iota
	CH_FAILED              CHStatus = iota
	CH_NO_CHANGE_REQUESTED CHStatus = iota
)

var (
	changes   = flag.BoolP("changes", "c", false, "verbose but for changes")
	deref     = flag.Bool("dereference", true, "affect sym link referent")
	noderef   = flag.BoolP("no-dereference", "h", false, "affect sym link rather than linked file")
	from      = flag.String("from", "", "change owner and/or group if owner/group matches. Either may be omitted.")
	npr       = flag.Bool("no-preserve-root", true, "don't treat root '/' specially")
	pr        = flag.Bool("preserve-root", false, "fail recursive operation on '/")
	silent    = flag.BoolP("silent", "f", false, "suppress most error messages")
	silent2   = flag.Bool("quiet", false, "suppress most error messages")
	rfile     = flag.String("reference", "", "use RFILE's owner/group")
	recursive = flag.BoolP("recursive", "R", false, "operate recursively")
	verbose   = flag.BoolP("verbose", "v", false, "diagnostic for each file")
	travDir   = flag.BoolP("N1O1L1O1N1G1O1P1T1", "H", false, "if cli arg is sym link to dir, follow it")
	travAll   = flag.BoolP("N1O1L1O1N1G1O1P1T2", "L", false, "traverse every sym link")
	noTrav    = flag.BoolP("N1O1L1O1N1G1O1P1T3", "P", true, "don't traverse any sym links")
	version   = flag.Bool("version", false, "print program's version\n")

	optUid = -1 // Specified uid; -1 if not to be changed.
	optGid = -1 // Specified gid; -1 if not to be changed.

	// Change the owner (group) of a file only if it has this uid (gid).
	// -1 means there's no restriction.
	reqUid = -1
	reqGid = -1

	mute = *silent || *silent2

	DO_NOT_FOLLOW = false
	// I used to try to curb infine symlink loops when -R was used, but
	// as it turns out Go just loops a lot and then quits for some reason
	// since this does, essentially, the same thing as testing for infinite
	// looping, I'm gonna leave it for right now.
	// I mean, it hasn't hung my system... yet.
	//anchor        = make(map[uint64]bool)

	ROOT_INODE uint64

	SkipDir  = errors.New("skip this directory")
	CantFind = errors.New("can't find user/group/uid/gid")
)

func nameToUid(name string) (int, error) {
	u, err := user.Lookup(name)
	if err != nil {
		return -2, CantFind
	}
	i, _ := strconv.Atoi(u.Uid)
	return i, nil
}

func nameToGid(name string) (int, error) {
	g, err := user.LookupGroup(name)
	if err != nil {
		return -2, CantFind
	}
	i, _ := strconv.Atoi(g.Gid)
	return i, nil
}

func gidToName(gid uint32) (string, error) {
	g, err := user.LookupGroupId(strconv.FormatUint(uint64(gid), 10))
	if err != nil {
		return "", CantFind
	}
	return g.Name, nil
}

func uidToName(uid uint32) (string, error) {
	u, err := user.LookupId(strconv.FormatUint(uint64(uid), 10))
	if err != nil {
		return "", CantFind
	}
	return u.Username, nil
}

func UserGroupStr(user, group string) string {
	var spec string
	if user != "" {
		if group != "" {
			spec = fmt.Sprintf("%s:%s", user, group)
		} else {
			spec = fmt.Sprintf("%s", user)
		}
	} else if group != "" {
		spec = fmt.Sprintf("%s", group)
	}
	return spec
}

func walk(path string, info os.FileInfo, uid, gid, reqUid, reqGid int) bool {
	var ok bool
	var fileInfo os.FileInfo
	var err error
	var fileIsSym bool
	followSym := false

	stat_t := syscall.Stat_t{}
	err = syscall.Stat(path, &stat_t)

	/*if _, ok := anchor[stat_t.Ino]; ok {
		fmt.Print("we've found a loop! Now exiting because loops are bad.\n")
		os.Exit(1)
	}*/

	if info.Mode()&os.ModeSymlink == os.ModeSymlink {
		fileIsSym = true
	}

	if DO_NOT_FOLLOW {
		return false
	}

	ok = ChangeOwner(path, info, uid, gid, reqUid, reqGid)

	names, err := readDirNames(path)
	if err != nil {
		return false
	}

	for _, name := range names {
		filename := filepath.Join(path, name)
		if !*deref {
			fileInfo, err = os.Lstat(filename)
			if fileIsSym {
				followSym = true
			}
		} else {
			fileInfo, err = os.Stat(filename)
		}

		if err != nil {
			ok = false
		} else {

			// If we're going to chown the symlink instead of the linked file,
			// we need to do it before we follow the link and continue with
			// our recursive chowning
			if followSym && *travAll || *travDir { //&& fileInfo.IsDir() {
				_ = ChangeOwner(filename, fileInfo, uid, gid, reqUid, reqGid)
				filename, _ = os.Readlink(filename)
				ok = walk(filename, fileInfo, uid, gid, reqUid, reqGid)
			} else if fileInfo.IsDir() {
				ok = walk(filename, fileInfo, uid, gid, reqUid, reqGid)
			} else {
				ok = ChangeOwner(filename, fileInfo, uid, gid, reqUid, reqGid)
			}
		}
	}
	return ok
}

// from http://golang.org/src/pkg/path/filepath/path.go
func readDirNames(dirname string) ([]string, error) {
	f, err := os.Open(dirname)
	if err != nil {
		return nil, err
	}
	names, err := f.Readdirnames(-1)
	f.Close()
	if err != nil {
		return nil, err
	}
	sort.Strings(names)
	return names, nil
}

// Returns true if chown is successful on all files
func ChownFiles(fname string, uid, gid, reqUid, reqGid int) bool {
	ok := false

	if !*deref {
		fi, err := os.Lstat(fname)
		if err != nil {
			if !mute {
				fmt.Printf("cannot lstat() file or directory '%s'\n", fname)
			}
		}
		if *recursive && fi.Mode()&os.ModeSymlink != os.ModeSymlink {
			if walk(fname, fi, uid, gid, reqUid, reqGid) {
				ok = true
			}
		} else {
			if ChangeOwner(fname, fi, uid, gid, reqUid, reqGid) {
				ok = true
			}
		}
	} else {
		fi, err := os.Stat(fname)
		if err != nil {
			if !mute {
				fmt.Printf("cannot stat() file or directory '%s'\n", fname)
			}
		}
		if *recursive {
			if walk(fname, fi, uid, gid, reqUid, reqGid) {
				ok = true
			}
		} else {
			if ChangeOwner(fname, fi, uid, gid, reqUid, reqGid) {
				ok = true
			}
		}
	}

	return ok
}

func ChangeOwner(fname string, origStat os.FileInfo, uid, gid, reqUid, reqGid int) bool {
	var status RCStatus
	var doChown bool
	var changed bool
	var changeStatus CHStatus

	symlinkChanged := true
	ok := true

	fi, err := os.Open(fname)
	if err != nil {
		if !mute {
			fmt.Printf("%v %s\n", err, fname)
		}
		ok = false
	}

	stat_t := syscall.Stat_t{}
	err = syscall.Stat(fname, &stat_t)

	// TODO: Better error messages, similar to fts(3)'s FTS_DNR, FTS_ERR,
	// and so on
	if err != nil {
		if !mute {
			fmt.Printf("%v %s\n", err, fname)
		}
		ok = false
	}

	// Check if we've stumbled across a directory
	if stat_t.Mode&ModeDir != 0 && stat_t.Ino == ROOT_INODE {
		if *recursive && *pr {
			fmt.Print("cannot run chown on root directory (--preserve-root specified\n")
			DO_NOT_FOLLOW = true
			return false
		} else {
			// Regardless of whether or not -f is true, print the warning
			// because I figure it won't bug anybody and it's better to
			// let people know they're about to do something bad than
			// have them do it without knowing!
			fmt.Print("warning: running chown on root directory without protection\n")
		}
	}

	if !ok {
		doChown = false
	} else {
		doChown = true
	} /*else if reqUid == -1 && reqGid == -1 && !*verbose && !*deref {
		doChown = true
	}*/

	if doChown {
		if !*deref {
			err := os.Lchown(fname, uid, gid)

			// GNU's chown says it ignores any error due to lack of support.
			// Apparently "POSIX requires this behavior for any top-level sym
			// links with -h, and implies it's required for all symlinks."
			if e, k := err.(*os.PathError); k && e.Err == syscall.EOPNOTSUPP {
				ok = true
				symlinkChanged = false
			} else if e, k := err.(*os.PathError); k && e.Err == syscall.EPERM || e.Err == syscall.EACCES {
				if !mute {
					fmt.Printf("%s\n", err)
				}
				ok = false
			} else if err != nil {
				ok = true
			} else {
				ok = false
			}
		} else {
			// Double check the size of the fd because the Go's openat() wants
			// the cwd_fd to be of the int type, while os.File's Fd() returns
			// a uintptr() of the fd. Theoretically, this means that the value
			// returned from Fd() could be out of the bounds of our int, thus
			// causing us to accidentally chown incorrectly.
			//
			// os.File.Fd() is the only way that I know how to get the fd without
			// actually using openat() -- which we can't do without a fd
			//
			// Apparently there's a soft restriction of ~4 billion inodes
			// which is set when the filesystem is created
			// Since int in Go is >= 32 bits, we have a range of
			// -2147483648 through 2147483647, which means we have about 1.8
			// billion inodes that we cannot (assuming 32 bit int size) run
			// RestrictedChown() on. That said, I'd be willing to bet most
			// systems do not have > 2147483647 inodes. If a system does,
			// we'll have to hope that said system is using 64 bit ints.
			// (Which is becoming more and more common.)
			if fi.Fd() <= uintptr(MAX_INT) {
				status = RestrictedChown(int(fi.Fd()), fname, origStat, uid, gid, reqUid, reqGid)
			} else {
				log.Fatalln("Go sucks, use C (just kidding)")
			}

			switch status {
			case RC_OK:
				break
			case RC_DO_ORDINARY_CHOWN:
				if !*deref {
					err := os.Lchown(fname, uid, gid)
					if err != nil {
						ok = false
						if os.IsPermission(err) {
							if !mute {
								fmt.Printf("%s\n", err)
							}
						}
					} else {
						ok = true
					}
				} else {
					err := os.Chown(fname, uid, gid)
					if err != nil {
						ok = false
						if os.IsPermission(err) {
							if !mute {
								fmt.Printf("%s\n", err)
							}
						}
					} else {
						ok = true
					}
				}
			case RC_ERROR:
				ok = false
			case RC_INODE_CHANGED:
				fmt.Printf("inode changed during chown of '%s'\n", fname)
				fallthrough
			case RC_EXCLUDED:
				doChown = false
				ok = false
			default:
				log.Fatalln("Now how did this happen?")
			}
		}
	}

	if *verbose || *changes && !mute {
		if changed = doChown && ok && symlinkChanged &&
			!((uid == -1 || uint32(uid) == stat_t.Uid) &&
				(gid == -1 || uint32(gid) == stat_t.Gid)); changed || *verbose {

			if !ok {
				changeStatus = CH_FAILED
			} else if !symlinkChanged {
				changeStatus = CH_NOT_APPLIED
			} else if !changed {
				changeStatus = CH_NO_CHANGE_REQUESTED
			} else {
				changeStatus = CH_SUCCEEDED
			}

			oldUsr, _ := uidToName(stat_t.Uid)
			oldGroup, _ := gidToName(stat_t.Gid)
			newUsr, _ := uidToName(uint32(optUid))
			newGrp, _ := gidToName(uint32(optGid))

			DescribeChange(fname, changeStatus, oldUsr, oldGroup, newUsr, newGrp)
		}
	}
	return ok
}

func RestrictedChown(cwd_fd int, file string, origStat os.FileInfo, uid, gid, reqUid, reqGid int) RCStatus {
	var status RCStatus

	openFlags := syscall.O_NONBLOCK | syscall.O_NOCTTY

	fstat := syscall.Stat_t{}
	err := syscall.Stat(file, &fstat)

	fileInfo, err := os.Stat(file)
	fileMode := fileInfo.Mode()

	if reqUid == -1 && reqGid == -1 {
		return RC_DO_ORDINARY_CHOWN
	}

	if !fileMode.IsRegular() {
		if fileMode.IsDir() {
			openFlags |= syscall.O_DIRECTORY
		} else {
			return RC_DO_ORDINARY_CHOWN
		}
	}

	fd, errno := syscall.Openat(cwd_fd, file, syscall.O_RDONLY|openFlags, 0)

	if !(0 <= fd || errno != nil && fileMode.IsRegular()) {
		if fd, err = syscall.Openat(cwd_fd, file, syscall.O_WRONLY|openFlags, 0); !(0 <= fd) {
			if err == syscall.EACCES {
				return RC_DO_ORDINARY_CHOWN
			} else {
				return RC_ERROR
			}
		}
	}

	if err := syscall.Fstat(fd, &fstat); err != nil {
		status = RC_ERROR
	} else if !os.SameFile(origStat, fileInfo) {
		status = RC_INODE_CHANGED
	} else if (reqUid == -1 || uint32(reqUid) == fstat.Uid) && (reqGid == -1 || uint32(reqGid) == fstat.Gid) { // Sneaky chown lol
		if err := syscall.Fchown(fd, uid, gid); err == nil {
			if err := syscall.Close(fd); err == nil {
				return RC_OK
			} else {
				return RC_ERROR
			}
		} else {
			if os.IsPermission(err) {
				fmt.Printf("%s\n", err)
			}
			status = RC_ERROR
		}
	}
	err = syscall.Close(fd)
	if err != nil {
		log.Fatalln(err)
	}
	return status
}

func DescribeChange(file string, changed CHStatus, olduser, oldgroup, user, group string) {

	userbool := false
	groupbool := false

	if user != "" {
		userbool = true
	} else {
		olduser = ""
	}
	if group != "" {
		groupbool = true
	} else {
		oldgroup = ""
	}

	if changed == CH_NOT_APPLIED {
		fmt.Printf("neither symbolic link '%s' nor referent has been changed\n", file)
	}

	spec := UserGroupStr(user, group)
	oldspec := UserGroupStr(olduser, oldgroup)

	switch changed {
	case CH_SUCCEEDED:
		if userbool {
			fmt.Printf("changed ownership of '%s' from %s to %s\n", file, oldspec, spec)
		} else if groupbool {
			fmt.Printf("changed group of '%s' from %s to %s\n", file, oldspec, spec)
		} else {
			fmt.Printf("no change in ownership of %s\n", file)
		}
	case CH_FAILED:
		if oldspec != "" {
			if userbool {
				fmt.Printf("failed to change ownership of '%s' from %s to %s\n", file, oldspec, spec)
			} else if groupbool {
				fmt.Printf("failed to change group of '%s' from %s to %s\n", file, oldspec, spec)
			} else {
				fmt.Printf("failed to change ownership of %s\n", file)
			}
		} else {
			if userbool {
				fmt.Printf("failed to change ownership of '%s' from %s to %s\n", file, oldspec, spec)
			} else if groupbool {
				fmt.Printf("failed to change group of '%s' from %s to %s\n", file, oldspec, spec)
			} else {
				fmt.Printf("failed to change ownership of %s\n", file)
			}
			oldspec = spec
		}
	case CH_NO_CHANGE_REQUESTED:
		if userbool {
			fmt.Printf("ownership of '%s' retained as %s\n", file, oldspec)
		} else if groupbool {
			fmt.Printf("ownership of '%s' retained as %s\n", file, oldspec)
		} else {
			fmt.Printf("ownership of '%s' retained\n", file)
		}
	default:
		log.Fatalln("let's go out with a bang!") // TODO: Good error messages lol
	}
}

func DetermineInput(input string, user bool) int {
	if input == "" {
		return -1
	} else if user {
		if id, err := nameToUid(input); err == nil {
			return id
		} else {
			if _, err = uidToName(uint32(id)); err == nil {
				return id
			}
		}
	} else if !user {
		if id, err := nameToGid(input); err == nil {
			return id
		} else {
			if _, err = uidToName(uint32(id)); err == nil {
				return id
			}
		}
	} else {
		// Basically if the user/uid/group/gid aren't found *and* the input
		// isn't -1, error out.
		if user {
			fmt.Fprintf(os.Stderr, "invalid username/uid %s\n", input)
		} else {
			fmt.Fprintf(os.Stderr, "invalid groupame/gid %s\n", input)
		}
		os.Exit(1)
	}
	log.Fatalln("We shouldn't be here right now.")
}

// We have to do extra arg parsing here because chown doesn't use the
// standard CLI format that other utilities do
// For instance...
// chown -R eric:root /home/eric/documents
//
// compared to...
//
// grep -r 'regex'
//
// Our flags parser (pflag) *can* handle this format, but need to split the
// string(s) (e.g. eric:root -> args[0] == eric && args[1] == root)
func main() {
	shopts := false // Short opts if *rfile
	ok := false

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "%s\n", HELP)
		os.Exit(0)
	}

	flag.Parse()

	if *version {
		fmt.Fprintf(os.Stderr, "%s\n", VERSION)
		os.Exit(0)
	}

	if *rfile != "" {
		shopts = true
	}

	if flag.NArg() < 2 || shopts && flag.NArg() < 1 {
		if flag.NArg() == 0 {
			fmt.Print("chown: missing operand\n")
		} else {
			fmt.Printf("chown: missing operand after '%s'\n", flag.Arg(0))
		}
		log.Fatalln(EXIT_FAILURE)
	}

	if *recursive && *deref && !*travAll && *noTrav {
		log.Fatalln("-R --dereference requires either -H or -L")
	}

	if shopts {
		stat_t := syscall.Stat_t{}
		err := syscall.Stat(*rfile, &stat_t)
		if err != nil {
			log.Fatalf("failed to get attributes of '%s'\n", *rfile)
		}
		optUid = int(stat_t.Uid)
		optGid = int(stat_t.Gid)
	} else {
		idArr := strings.Split(flag.Args()[0], ":")
		optUid = DetermineInput(idArr[0], true)
		optGid = DetermineInput(idArr[1], false)
	}

	if *from != "" {
		idArr := strings.Split(*from, ":")
		reqUid = DetermineInput(idArr[0], true)
		reqGid = DetermineInput(idArr[1], false)
	}

	if *recursive && *pr {
		stat_t := syscall.Stat_t{}
		if err := syscall.Stat("/", &stat_t); err != nil {
			log.Fatalf("failed to get attributes of %q\n", "/")
		}
		ROOT_INODE = stat_t.Ino
	}

	if shopts {
		for _, file := range flag.Args()[2:] {
			ok = ChownFiles(file, optUid, optGid, reqUid, reqGid)
		}
	} else {
		for _, file := range flag.Args()[1:] {
			ok = ChownFiles(file, optUid, optGid, reqUid, reqGid)
		}
	}
	if !ok {
		os.Exit(1)
	}
}
