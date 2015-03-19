package main

import (
	"fmt"
	"os"
	"path/filepath"
	//"strings"
	"errors"
	"sort"
	"syscall"
)

const (
	MAXPATHLEN = 1024
)

const (
	_ = iota
	CHILD
	NAMES
	READ
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
	// FTS.Options
	COMFOLLOW  = 0x001 // follow symlinks
	LOGICAL    = 0x002 // logical walk
	NOCHDIR    = 0x004 // don't change dir
	NOSTAT     = 0x008 // don't get stat() info
	PHYSICAL   = 0x010 // physical walk
	SEEDOT     = 0x020 // return dot and dot-dot
	XDEV       = 0x040 // don't cross devices
	WHITEOUT   = 0x080 // return whiteout information
	OPTIONMASK = 0x0ff // valid user option mask
	NAMEONLY   = 0x100 // (private) child names only
	STOP       = 0x200 // (private) unrecoverable error
)

const (
	// FTSENT.Level
	ROOTPARENTLEVEL = -1 // pre
	ROOTLEVEL       = 0  // 0th
)

const (
	// FTSENT.Info
	_       = iota // Skip 0
	D              // preorder dir
	DC             // directory that causes cycles
	DEFAULT        // none of the above
	DNR            // unreadable directory
	DOT            // dot or dot-dot
	DP             // postorder directory
	ERR            // error; errno is set
	F              // regular file
	INIT           // initialized only
	NS             // stat() failed
	NSOK           // no stat() requested
	SL             // symlink
	SLONNE         // symlink w/o target
	W              // whiteout object
)

const (
	// FTSENT.Flags
	DONTCHDIR = 0x01 // don't chdir to parent
	SYMFOLLOW = 0x02 // followed a symlink to get here
	ISW       = 0x04 // this is a whiteout object
)

const (
	// FTSENT.Inst
	_       = iota // skip 0
	AGAIN          // read node again
	FOLLOW         // follow symlink
	NOINSTR        // no instructions
	SKIP           // discard node
)

type FTS struct {
	Cur     *FTSENT  // current node
	Child   *FTSENT  // linked list of childrne
	Array   **FTSENT // sort array
	Dev     uint64   // starting device #
	Path    string   // path for this descent
	Options int      // FTSOpen() options
}

type FTSENT struct {
	Cycle      *FTSENT         // cycle node
	Parent     *FTSENT         // parent directory
	Child      *FTSENT         // next file in directory
	Number     uint64          // local numeric value
	Pointer    *interface{}    // local address value
	AccessPath string          // access path
	Path       string          // root path
	Errno      int             // errno for this node
	Ino        uint64          // inode
	Dev        uint64          // device #
	NLink      uint64          // link count
	Level      int             // depth, -1 to N
	Info       int             // user flags for FSENT struct
	Stat       *syscall.Stat_t // syscall.Stat() information
	Name       string          // file name

}

var (
	LastNode = errors.New("end of node")
)

func (f *FTSENT) next() *FTSENT {
	return f.Child
}

func (f *FTS) isLoop(stat syscall.Stat_t) bool {
	ino := stat.Ino
	cur := f.Child

	// 1st in linked list
	if cur.Ino == ino {
		return false
	}

	// 2nd through end
	for {
		if cur.next().Ino == ino {
			return false
		}
	}
	return true
}

func (f *FTS) isSet(opt int) bool {
	if f.Options&opt == 0 {
		return true
	}
	return false
}

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

func NewNode(fname string, parent *FTSENT) (*FTSENT, error) {
	stat_t := syscall.Stat_t{}
	err := syscall.Stat(fname, &stat_t)
	if err != nil {
		return nil, err
	}

	if stat_t.Ino == parent.Ino {
		return nil, LastNode
	}

	level := parent.Level + 1
	_, relPath := filepath.Split(fname)
	return &FTSENT{
		Parent:     parent,
		AccessPath: relPath,
		Path:       fname,
		Errno:      0,
		Ino:        stat_t.Ino,
		Dev:        stat_t.Dev,
		NLink:      stat_t.Nlink,
		Level:      level,
		Stat:       &stat_t,
		Name:       filepath.Base(fname),
	}, nil
}

// Open() returns a pointer to an FTS struct giving information about
// the file hierarchy for traversal.
// Open() takes a path to a file and options for traversing the file
// hierarchy.
// e.g. fh, err := Open("/home/eric/music", PHYSICAL|XDEV)
func Open(path string, options int) (*FTS, *FTSENT, error) {

	stat_t := syscall.Stat_t{}
	err := syscall.Stat(path, &stat_t)

	if err != nil {
		return nil, nil, err
	}

	root := FTSENT{
		Cycle:      nil,
		Parent:     nil,
		Child:      nil,
		Number:     0,
		Pointer:    nil,
		AccessPath: path,
		Path:       path,
		Errno:      0,
		Ino:        stat_t.Ino,
		Dev:        stat_t.Dev,
		NLink:      stat_t.Nlink,
		Level:      ROOTLEVEL,
		Info:       DEFAULT,
		Stat:       &stat_t,
		Name:       filepath.Base(path),
	}

	sp := FTS{
		Cur:   &root,
		Child: &root,
		Dev:   stat_t.Dev,
		Path:  path,
	}

	// Options check
	if options & ^OPTIONMASK != 0 {
		return nil, nil, syscall.EINVAL
	}

	sp.Options = options

	// Logical walks turn on NOCHDIR; symlinks are too hard
	if sp.isSet(LOGICAL) {
		sp.Options |= NOCHDIR
	}

	return &sp, &root, nil
}

// Builds a linked list of files in the provided heirarchy
func Build(sp *FTS) (*FTSENT, error) {
	root := sp.Child

	f, err := os.Open(root.Path)
	if err != nil {
		return nil, err
	}
	fi, err := f.Readdir(-1)

	opts := sp.Options

}

func main() {
	sp, root, err := Open("/home/eric/github-repos/go-coreutils/", PHYSICAL|XDEV)
	if err != nil {
		fmt.Printf("%v\n", err)
		os.Exit(1)
	}
	f, err := Build(sp, root, true, READ)
	if err != nil {
		panic(err)
	} else {
		fmt.Printf("%+v", f)
	}
	//fmt.Println("Hello, world!")
}
