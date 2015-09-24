/*
    go md5sum

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

package main

import (
	"bufio"
	flag "github.com/ogier/pflag"
	"io"
	"os"
	"path/filepath"
	"strings"
)

/*
   check the md5sum for all of files from cmdline
*/
func check_md5sum() bool {
	if len(flag.Args()) == 0 ||
		(len(flag.Args()) == 1 && flag.Args()[0] == "-") {

		/* Known Issue:
		   Ctrl+Z on cmd can not trigger io.EOF */
		return check_md5sum_f(os.Stdin)
	}

	has_err := false

	for i := 0; i < len(flag.Args()); i++ {
		//output_e("use file: %s\n", flag.Args()[i])
		file, err := os.Open(flag.Args()[i])
		if err != nil {
			output_e("md5sum: %s\n", err.Error())
			has_err = true
			continue
		}
		if b := check_md5sum_f(file); !b {
			has_err = true
		}
		file.Close()
	}

	return !has_err
}

/*
   process single md5 list file
*/
func check_md5sum_f(fp io.Reader) bool {
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
				output_e("md5sum: %s\n", err.Error())
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
			if *show_warn {
				output_e("md5sum: line: %d: improperly formatted MD5 checksum line\n",
					line_num)
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
			output_e("md5sum: %s\n", err.Error())
			has_err = true
			errored += 1
			continue
		}

		sum1 := calc_md5sum(file)
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

	if failed > 0 && *show_warn {
		output_e("md5sum: WARNING: %d of %d computed checksums did NOT match\n",
			failed, total)
	}

	if errored > 0 && *show_warn {
		output_e("md5sum: WARNING: %d of %d listed files could not be read\n",
			errored, total)
	}

	return !has_err
}
