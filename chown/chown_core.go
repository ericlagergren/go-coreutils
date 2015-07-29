package main

import (
	"fmt"
	"os"
	"syscall"
	// "strconv"
)

func newChownOption() *ChownOption {
	// Rest fall back to their zero-value.
	return &ChownOption{
		verbosity:             VOff,
		affectSymlinkReferent: true,
	}
}

// func gidToName(gid uint32) (string, error) {
// 	id, err := user.LookupGroupId(strconv.FormatUint(uint64(gid), 10))
// 	if err != nil {
// 		return "", err
// 	}
// 	return id.Name, nil
// }

// func uidToName(uid uint32) (string, error) {
// 	id, err := user.LookupId(strconv.FormatUint(uint64(uid), 10))
// 	if err != nil {
// 		return "", err
// 	}
// 	return id.Name, nil
// }

func UserGroupString(user, group string) string {
	var spec string
	if user != "" {
		if group != "" {
			spec = fmt.Sprintf("%s:%s", user, group)
		} else {
			spec = user
		}
	} else if group != "" {
		spec = group
	}
	return spec
}

func DescribeChange(file string, changed ChangeStatus, oldUser, oldGroup, user, group string) {

	if changed == CHNotApplied {
		fmt.Printf("neither symbolic link %q nor referent has been changed\n", file)
		return
	}

	spec := UserGroupString(user, group)
	var oldSpec string
	if user != "" {
		if group != "" {
			oldSpec = UserGroupString(oldUser, oldGroup)
		} else {
			oldSpec = UserGroupString(oldUser, "")
		}
	} else if group != "" {
		oldSpec = UserGroupString("", oldGroup)
	} else {
		oldSpec = UserGroupString("", "")
	}

	var format string
	switch changed {
	case CHSucceeded:
		if user != "" {
			format = "changed ownership of %q from %s to %s\n"
		} else if group != "" {
			format = "changed group of %q from %s to %s\n"
		} else {
			format = "no change to ownership of %s\n"
		}
	case CHFailed:
		if oldSpec != "" {
			if user != "" {
				format = "failed to change ownership of %q from %s to %s\n"
			} else if group != "" {
				format = "failed to change group of %q from %s to %s\n"
			} else {
				format = "failed to change ownership of %q\n"
			}
		}
	case CHNoChangeRequested:
		if user != "" {
			format = "ownership of %q retained as %s\n"
		} else if group != "" {
			format = "group of %q retained as %s\n"
		} else {
			format = "ownership of %q retained\n"
		}
	default:
		os.Exit(1)
	}

	fmt.Printf(format, file, oldSpec, spec)
}

func RestrictedChown(cwdFd int, file string, origStat os.FileInfo, uid, gid, reqUid, reqGid int) RCHStatus {

	status := RCOK
	openflags := syscall.O_NONBLOCK | syscall.O_NOCTTY

	if reqUid == -1 && reqGid == 1 {
		return RCDoOrdinaryChown
	}

	if !origStat.Mode().IsRegular() {
		if !origStat.Mode().IsDir() {
			return RCDoOrdinaryChown
		}
		openflags |= syscall.O_DIRECTORY
	}

	fd, err := syscall.Openat(cwdFd, file, syscall.O_RDONLY|openflags, 0)

	if !(0 <= fd ||
		err == syscall.EACCES &&
			origStat.Mode().IsRegular()) {

		if fd, err = syscall.Openat(cwdFd, file, syscall.O_WRONLY|openflags, 0); 0 > fd {

			if err == syscall.EACCES {
				return RCDoOrdinaryChown
			}
			return RCError
		}
	}

	var stat syscall.Stat_t
	if err := syscall.Fstat(fd, &stat); err != nil {
		status = RCError
	} else if !sameFile(origStat.Sys().(*syscall.Stat_t), &stat) {
		status = RCInodeChanged
	} else if reqUid == -1 || uint32(reqUid) == stat.Uid &&
		(reqGid == -1 || uint32(reqGid) == stat.Gid) {
		if syscall.Fchown(fd, uid, gid) == nil {
			if syscall.Close(fd) == nil {
				return RCOK
			}
			return RCError
		} else {
			status = RCError
		}
	}

	return status
}

// Basically borrowed from os/stat_linux.go but there's no
// other way to do it.
func sameFile(st1, st2 *syscall.Stat_t) bool {
	return st1.Dev == st2.Dev && st1.Ino == st2.Ino
}
