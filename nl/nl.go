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
import "strings"
import flag "github.com/ogier/pflag"

func main() {
	bFlag := flag.StringP("body-numbering", "b", "t", "style")
	flag.Parse()
	if len(flag.Args()) == 0 {
		bytes, _ := ioutil.ReadAll(os.Stdin)
		lines := strings.Split(string(bytes), "\n")
		linecount := 0
		for i := 0; i < len(lines)-1; i++ {
			if *bFlag == "t" {
				if len(strings.TrimSpace(lines[i])) > 0 {
					linecount++
					fmt.Printf("%6d  %s\n", linecount, lines[i])
				} else {
					fmt.Println(lines[i])
				}
			} else {
				linecount++
				fmt.Printf("%6d  %s\n", linecount, lines[i])
			}
		}
	} else if len(flag.Args()) > 0 {
		linecount := 0
		for j := 0; j < len(flag.Args()); j++ {
			bytes, _ := ioutil.ReadFile(flag.Arg(j))
			lines := strings.Split(string(bytes), "\n")
			for i := 0; i < len(lines)-1; i++ {
				if *bFlag == "t" {
					if len(strings.TrimSpace(lines[i])) > 0 {
						linecount++
						fmt.Printf("%6d  %s\n", linecount, lines[i])
					} else {
						fmt.Println(lines[i])
					}
				} else {
					linecount++
					fmt.Printf("%6d  %s\n", linecount, lines[i])
				}
			}
		}
	}
}
