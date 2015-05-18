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

import (
  "flag"
  "fmt"
  "path"
)

func basename(args []string) string {
  var base string

  if len(args) == 1 {
    base = path.Base(args[0])
  } else if len(args) == 2 {
    base = path.Base(args[0])
    suffix := args[1]
    if base[len(base)-len(suffix):] == suffix {
      base = base[:len(base)-len(suffix)]
    }
  }

  return base
}

func main() {
  flag.Parse()
  name := basename(flag.Args())
  fmt.Println(name)
}
