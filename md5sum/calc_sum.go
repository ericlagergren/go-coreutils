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
	"crypto/md5"
	"fmt"
	flag "github.com/ogier/pflag"
	"io"
	"os"
	"path/filepath"
)

/*
   read from os.File and return the whole file's md5sum
*/
func calc_md5sum(fp io.Reader) string {
	m := md5.New()

	/*  issue:
	    if fp is os.Stdin, there is no way to trigger EOF
	*/
	_, err := io.Copy(m, fp)

	if err != nil {
		output_e("md5sum: %s\n", err.Error())
		return ""
	}

	return fmt.Sprintf("%x", m.Sum(nil))
}

/*
   generate the md5sum for all of files from cmdline
*/
func gen_md5sum() bool {
	if len(flag.Args()) == 0 ||
		(len(flag.Args()) == 1 && flag.Args()[0] == "-") {

		/* Known Issue:
		   Ctrl+Z on cmd can not trigger io.EOF */
		sum := calc_md5sum(os.Stdin)

		fmt.Fprintf(os.Stdout, "%s *%s\n", sum, "-")
		return true
	}

	has_error := false

	for i := 0; i < len(flag.Args()); i++ {
		fn := flag.Args()[i]

		/* extends file lists when filename contains '*' */
		filenames, _ := filepath.Glob(fn)
		if filenames == nil {
			filenames = append(filenames, fn)
		}

		for _, f := range filenames {
			file, err := os.Open(f)
			if err != nil {
				has_error = true
				fmt.Fprintf(os.Stderr, "md5sum: %s\n", err.Error())
				continue
			}
			sum := calc_md5sum(file)
			file.Close()
			if sum != "" {
				has_error = true
				fmt.Fprintf(os.Stdout, "%s *%s\n", sum, f)
			}
		}
	}

	return !has_error
}
