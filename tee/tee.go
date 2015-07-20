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

import "os"
import "fmt"
import "io/ioutil"
import flag "github.com/ogier/pflag"
import "log"

func main() {
	flagAppend := flag.BoolP("append", "a", false, "append to file")
	flag.Parse()
	bytes, _ := ioutil.ReadAll(os.Stdin)
	for i := 0; i < len(flag.Args()); i++ {
		if *flagAppend {
			f, err := os.OpenFile(flag.Args()[i], os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
			if err != nil {
				log.Fatal(err)
			}
			f.Write(bytes)
			f.Close()
		} else {
			f, err := os.OpenFile(flag.Args()[i], os.O_WRONLY|os.O_CREATE, 0644)
			if err != nil {
				log.Fatal(err)
			}
			f.Write(bytes)
			f.Close()
		}
	}
	fmt.Printf("%s", string(bytes))
}
