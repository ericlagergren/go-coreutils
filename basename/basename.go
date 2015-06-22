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
	"fmt"
	"path"
	"strings"

	flag "github.com/ogier/pflag"
)

var (
	multiple = flag.StringP("multiple", "a", "", "file ...")
	suffix   = flag.StringP("suffix", "s", "", ".h")
	zero     = flag.BoolP("zero", "z", false, "")
)

func performBasename(s, suffix string, null bool) string {
	base := path.Base(s)

	// if base[len(base)-len(suffix):] == suffix {
	if strings.HasSuffix(base, suffix) { // let's justify strings package presence :)
		base = base[:len(base)-len(suffix)]
	}

	if !null {
		base += "\n"
	}

	return base
}

func main() {
	flag.Parse()

	switch {
	case *multiple != "":
		// ugly things about to happen...
		if flag.NArg() > 0 { // means it's using -a
			fmt.Print(performBasename(*multiple, *suffix, *zero)) // pick first from -a

			for _, v := range flag.Args() { // the rest...
				fmt.Print(performBasename(v, *suffix, *zero))
			}

		} else { // means it's using --multiple, split em all
			for _, v := range strings.Split(*multiple, " ") {
				fmt.Print(performBasename(v, *suffix, *zero))
			}
		}
	case *suffix != "": //implies -a
		fmt.Print(performBasename(flag.Args()[0], *suffix, *zero))
	default:
		name := flag.Args()[0]
		suffix := ""
		if flag.NArg() == 2 {
			suffix = flag.Args()[1]
		}
		fmt.Print(performBasename(name, suffix, *zero))
	}

}
