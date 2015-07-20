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
import flag "github.com/ogier/pflag"
import "time"
import "log"

func main() {
	cFlag := flag.BoolP("no-create", "c", false, "do not create file")
	flag.Parse()
	if len(flag.Args()) > 0 {
		for i := 0; i < len(flag.Args()); i++ {
			filename := flag.Arg(i)
			_, err := os.Stat(filename)
			if err == nil {
				now := time.Now()
				os.Chtimes(filename, now, now)
			} else {
				if !(*cFlag) {
					f, err := os.OpenFile(filename, os.O_WRONLY|os.O_CREATE, 0644)
					f.Close()
					if err != nil {
						log.Fatal(err)
					}
				}
			}
		}
	}
}
