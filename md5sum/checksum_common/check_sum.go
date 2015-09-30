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

/*
the core checksum implements of md5, sha1, sha224, sha256, sha384, sha512.
*/
package checksum_common

import (
	"bufio"
	//flag "github.com/ogier/pflag"
	"io"
	"os"
	"path/filepath"
	"strings"
)

/*
   check the checksum for all of files
*/
func check_checksum(files []string, t string) bool {

	has_err := false

	for i := 0; i < len(files); i++ {

		/* stdin */
		if files[i] == "-" {
			if b := check_checksum_f(os.Stdin, t); !b {
				has_err = true
			}
			continue
		}

		/* file */
		file, err := os.Open(files[i])
		if err != nil {
			output_e("%ssum: %s\n", t, err.Error())
			has_err = true
			continue
		}
		if b := check_checksum_f(file, t); !b {
			has_err = true
		}
		file.Close()
	}

	return !has_err
}

/*
   process single checksum list file
*/
func check_checksum_f(fp io.Reader, t string) bool {
	has_err := false
	reader := bufio.NewReader(fp)

	/* total file */
	total := 0

	/* failed number */
	failed := 0

	/* error number */
	errored := 0

	/* line number */
	line_num := 0

	for {
		line_num += 1
		l, _, err := reader.ReadLine()
		if err != nil {
			if err != io.EOF {
				has_err = true
				output_e("%ssum: %s\n", t, err.Error())
			}
			break
		}

		ll := strings.TrimSpace(string(l))

		if ll == "" {
			continue
		}

		/* strip the '\' at beginning */
		if ll[0] == '\\' {
			ll = ll[1:]
		}

		fields := strings.Fields(ll)

		if len(fields) != 2 {
			if show_warn {
				output_e("%ssum: line: %d: improperly formatted %s checksum line\n",
					t, line_num, strings.ToUpper(t))
			}
			continue
		}

		sum, fn := fields[0], fields[1]

		/* strip the '*' from filename */
		if fn[0] == '*' {
			fn = fn[1:]
		}

		fn = filepath.Clean(fn)

		file, err := os.Open(fn)
		if err != nil {
			output_e("%ssum: %s\n", t, err.Error())
			has_err = true
			errored += 1
			continue
		}

		sum1 := calc_checksum(file, t)
		file.Close()

		total += 1

		if sum1 != "" {
			if sum1 != sum { // failed
				failed += 1
				output_e("%s: FAILED\n", fn)
				has_err = true
			} else { // success
				output_n("%s: OK\n", fn)
			}
		} else { // error
			errored += 1
			has_err = true
		}
	}

	if failed > 0 && show_warn {
		output_e("%ssum: WARNING: %d of %d computed checksums did NOT match\n",
			t, failed, total)
	}

	if errored > 0 && show_warn {
		output_e("%ssum: WARNING: %d of %d listed files could not be read\n",
			t, errored, total)
	}

	return !has_err
}

/*
read the file contains the checksum and check it

files: file name lists which contains the checksums.

t: the type of checksum, md5 or sha1...

return true if everything is ok

return false if there are some errors.
*/
func CompareChecksum(files []string, t string, output_message, output_warn bool) bool {
	no_output = !output_message
	show_warn = output_warn
	return check_checksum(files, t)
}
