/*
    go checksum common

    Copyright (c) 2014-2015 Dingjun Fang

	This program is free software: you can redistribute it and/or modify
	it under the terms of the GNU General Public License version 3 as
	published by the Free Software Foundation.

	This program is distributed in the hope that it will be useful,
	but WITHOUT ANY WARRANTY; without even the implied warranty of
	MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
	GNU General Public License for more details.

	You should have received a copy of the GNU General Public License
	along with this program.  If not, see <http://www.gnu.org/licenses/>.
*/

package checksum_common

import (
	"fmt"
	"os"
)

var no_output bool = false
var show_warn bool = true

/*
   output to stdout
*/
func output_n(s string, s1 ...interface{}) {
	if no_output != true {
		fmt.Fprintf(os.Stdout, s, s1...)
	}
}

/*
   output to stderr
*/
func output_e(s string, s1 ...interface{}) {
	if no_output != true {
		fmt.Fprintf(os.Stderr, s, s1...)
	}
}
