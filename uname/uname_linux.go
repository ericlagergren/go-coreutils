/*
   Go uname -- print system information

   Copyright (c) 2014-2015  Eric Lagergren

   This program is free software: you can redistribute it and/or modify
   it under the terms of the GNU General Public License as published by
   the Free Software Foundation, either version 3 of the License, or
   (at your option) any later version.

   This program is distributed in the hope that it will be useful,
   but WITHOUT ANY WARRANTY; without even the implied warranty of
   MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
   GNU General Public License for more details.

   You should have received a copy of the GNU General Public License
   along with this program.  If not, see <http://www.gnu.org/licenses/>.  */

/*
	Written by Eric Lagergren <ericscottlagergren@gmail.com>
*/

package main

import "syscall"

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

func genInfo() (*info, error) {
	var name syscall.Utsname
	err := syscall.Uname(&name)
	return &info{
		sysname:  intToString(name.Sysname),
		nodename: intToString(name.Nodename),
		release:  intToString(name.Release),
		version:  intToString(name.Version),
		machine:  intToString(name.Machine),
	}, err
}
