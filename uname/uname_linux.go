// Copyright (c) 2014-2016 Eric Lagergren
// Use of this source code is governed by the GPL v3 or later.

package main

import "golang.org/x/sys/unix"

type info struct {
	sysname  string
	nodename string
	release  string
	version  string
	machine  string
}

func intToString(a [65]int8) string {
	var (
		tmp [65]byte
		i   int
	)
	for i = 0; a[i] != 0; i++ {
		tmp[i] = byte(a[i])
	}
	return string(tmp[:i])
}

func genInfo() (info, error) {
	var name unix.Utsname
	err := unix.Uname(&name)
	return info{
		sysname:  intToString(name.Sysname),
		nodename: intToString(name.Nodename),
		release:  intToString(name.Release),
		version:  intToString(name.Version),
		machine:  intToString(name.Machine),
	}, err
}
