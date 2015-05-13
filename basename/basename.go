/*
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

package main

import "fmt"
import "flag"
import "path"

func main() {
	flag.Parse()
	if len(flag.Args()) == 1 {
		fmt.Println(path.Base(flag.Arg(0)))
	} else if len(flag.Args()) == 2 {
		base   := path.Base(flag.Arg(0))
		suffix := flag.Arg(1)
		if base[len(base)-len(suffix):] == suffix {
			fmt.Println(base[:len(base)-len(suffix)])
		} else {
			fmt.Println(base)
		}
	}
}
