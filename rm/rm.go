package rm

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/ericlagergren/go-coreutils/rm/internal/sys"
	"golang.org/x/sys/unix"
)

type RemoveOption uint8

const (
	NoPreserveRoot = 1 << iota
	Force
	Recursive
	RemoveEmpty
	IgnoreMissing
	OneFileSystem
	Verbose
	PromptAlways
)

type PromptOption uint8

const (
	// WriteProtected signals that the object to be removed or descended upon
	// has write protection.
	WriteProtected PromptOption = 1 << iota
	// Directory indicates the action is on a directory.
	Directory
	// Descend indicates the action is to descend into an object.
	Descend
	// Remove indicates the action is to remove an object.
	Remove
)

func NewRemover(opts RemoveOption) *Remover {
	r := Remover{opts: opts}
	if opts&Verbose != 0 {
		r.Log = make(chan string)
	}
	return &r
}

type Remover struct {
	opts RemoveOption
	root os.FileInfo

	// Prompt, if non-nil, will be called depending on the Remover's configured
	// options. If it returns true, the action continues, otherwise it stops.
	Prompt func(name string, opts PromptOption) bool

	Log chan string

	stack []node
}

type node struct {
	path string
	info os.FileInfo
	kids int
}

func (r *Remover) Remove(path string) (err error) {
	r.root, err = os.Lstat(path)
	if err != nil {
		return err
	}

	if r.opts&Recursive == 0 || !r.root.Mode().IsDir() {
		if err := r.remove(path, r.root); err != nil && err != errRefused {
			return err
		}
		return nil
	}

	// GNU rm uses a DFS that, once it reaches a leaf node (doesn't contain any
	// further directories), clears out all files and "walks back" to the most
	// recently seen non-leaf node. This is typicall DFS behavior, but the
	// walking back is important: it allows the prompt for interactive usage to
	// look like this:
	//
	//  $ mkdir a/b/c
	//  $ touch a/b/c/d.txt
	//  $ rm a/
	//  rm: descend into 'a'?
	//  rm: descend into 'a/b'?
	//  rm: descend into 'a/b/c'?
	//  rm: remove file 'a/b/c/d.txt'?
	//  rm: remove directory 'a/b/c'?
	//  rm: remove directory 'a/b'?
	//  rm: remove directory 'a'?
	//
	// Unfortunately, filepath.Walk doesn't allow us to walk back, so we're
	// forced to do a little state management ourselves. We push each directory
	// we encounter onto a stack. Once we hit a leaf node, we manually work our
	// way back by popping every consecutive leaf node off the stack, removing
	// it as we go. Since filepath.Walk doesn't work backwards, this works.
	//
	// A major downside is the requirement of determining how many objects are
	// in a directory. This means Stat will be called twice for each directory:
	// once for filepath.Walk, once for us. Same goes for Readdirnames.
	return filepath.Walk(path, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			if !r.prompt(path, Descend) {
				return filepath.SkipDir
			}
			dir, err := os.Open(path)
			if err != nil {
				return err
			}
			files, err := dir.Readdirnames(-1)
			if err != nil {
				return err
			}
			r.stack = append(r.stack, node{path: path, info: info, kids: len(files)})
			return dir.Close()
		}

		err = r.remove(path, info)

		// Work our way down the r.stack.
		for i := len(r.stack) - 1; i >= 0; i-- {
			s := &r.stack[i]
			s.kids--
			if s.kids != 0 {
				r.stack = r.stack[:i+1]
				break
			}
			if err := r.remove(s.path, s.info); err != nil && err != errRefused {
				return err
			}
		}

		if err != nil && err != errRefused {
			return err
		}
		return nil
	})
}

var errRefused = errors.New("user refused prompt")

type rmError struct{ msg string }

func (r rmError) Error() string { return r.msg }

func (r *Remover) prompt(name string, opts PromptOption) bool {
	if r.opts&PromptAlways != 0 && r.Prompt != nil {
		return r.Prompt(name, opts)
	}
	return true
}

func (r *Remover) rm(name string, dir bool) (err error) {
	opts := Remove
	if dir {
		opts |= Directory
	}
	if !r.prompt(name, opts) {
		return errRefused
	}

	switch runtime.GOOS {
	case "windows", "plan9":
		err = os.Remove(name)
	default:
		// For unix systems, os.Remove is a call to Unlink followed by a call to
		// Rmdir. Since os.Remove doesn't know whether the object is a file or
		// directory, this provides better performance in the common case. But,
		// since we know the type of the object ahead of time, we can simply call
		// the proper syscall.
		if !dir {
			err = unix.Unlink(name)
		} else {
			err = unix.Rmdir(name)
		}
		if err != nil {
			err = &os.PathError{Op: "remove", Path: name, Err: err}
		}
	}
	if err != nil && (r.opts&IgnoreMissing == 0 || !os.IsNotExist(err)) {
		return err
	}
	if r.opts&Verbose != 0 {
		if dir {
			r.Log <- fmt.Sprintf("removed directory %s")
		} else {
			r.Log <- fmt.Sprintf("removed %s")
		}
	}
	return nil
}

func (r *Remover) remove(path string, info os.FileInfo) error {
	if info.Mode()&os.ModeDir != 0 {
		switch info.Name() {
		// POSIX doesn't let us do anything with . or ..
		case ".", "..":
			return rmError{msg: "cannot remove '.' or '..'"}
		case "/":
			return rmError{msg: "cannot remove root directory"}
		default:
			if r.opts&NoPreserveRoot == 0 && sys.IsRoot(info) {
				return rmError{msg: "cannot remove root directory"}
			}
		}
		if r.opts&Recursive == 0 && (r.opts&RemoveEmpty == 0 || isEmpty(path)) {
			return rmError{msg: fmt.Sprintf("cannot remove directory: %q", path)}
		}
		return r.rm(path, true)
	}
	if r.opts&OneFileSystem != 0 && sys.DiffFS(r.root, info) {
		return rmError{msg: "cannot recurse into a different filesystem"}
	}
	return r.rm(path, false)
}

func isEmpty(path string) bool {
	file, err := os.Open(path)
	if err != nil {
		return false
	}
	defer file.Close()
	names, err := file.Readdirnames(1)
	return len(names) != 0 && err == nil
}
