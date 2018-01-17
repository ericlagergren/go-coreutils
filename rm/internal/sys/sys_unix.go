// +build !windows

package sys

import (
	"os"
	"syscall"
)

var root *syscall.Stat_t

func init() {
	if info, err := os.Lstat("/"); err == nil {
		root = info.Sys().(*syscall.Stat_t)
	}
}

func IsRoot(info os.FileInfo) bool {
	stat := info.Sys().(*syscall.Stat_t)
	return root.Ino == stat.Ino && root.Dev == stat.Dev
}

func DiffFS(orig, test os.FileInfo) bool {
	return orig.Sys().(*syscall.Stat_t).Dev != test.Sys().(*syscall.Stat_t).Dev
}
