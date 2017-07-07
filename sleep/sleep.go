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

import "time"
import "os"
import "strconv"

func main() {
	if len(os.Args) >= 2 {
		duration, err := strconv.ParseInt(os.Args[1], 0, 64)
		if err != nil || duration < 0 {
			os.Exit(1)
		}
		time.Sleep(time.Duration(duration) * time.Second)
	}
}
