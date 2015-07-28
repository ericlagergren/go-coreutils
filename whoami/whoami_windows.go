/*
	Go whoami -- print effective userid

	Copyright (c) 2015 Eric Lagergren

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

/* Equivalent to 'id -un'. */
/* Written by Eric Lagergren and mattn */

package main

import "os/user"

func getUser() string {
	u, err := user.Current()
	// TODO(eric): Have this output match whoami_unix.go
	if err != nil {
		fatal.Fatalln("cannot find name for current user")
	}
	return u.Username
}
