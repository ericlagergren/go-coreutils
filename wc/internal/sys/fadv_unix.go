// +build linux freebsd

package sys

import "golang.org/x/sys/unix"

func Fadvise(fd int) error {
	return unix.Fadvise(fd, 0, 0, unix.FADV_SEQUENTIAL)
}
