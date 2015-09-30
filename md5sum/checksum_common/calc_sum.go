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

package checksum_common

import (
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"fmt"
	//flag "github.com/ogier/pflag"
	"hash"
	"io"
	"os"
	"path/filepath"
)

/*
   read from os.File and return the whole file's checksum
*/
func calc_checksum(fp io.Reader, t string) string {
	var m hash.Hash
	switch t {
	case "md5":
		m = md5.New()
	case "sha1":
		m = sha1.New()
	case "sha512":
		m = sha512.New()
	case "sha256":
		m = sha256.New()
	case "sha224":
		m = sha256.New224()
	case "sha384":
		m = sha512.New384()
	default:
		output_e("unknown type: %s\n", t)
		return ""
	}

	/*  issue:
	    if fp is os.Stdin, there is no way to trigger EOF
	*/
	_, err := io.Copy(m, fp)

	if err != nil {
		output_e("%ssum: %s\n", t, err.Error())
		return ""
	}

	return fmt.Sprintf("%x", m.Sum(nil))
}

/*
   generate the checksum for all of files from cmdline
*/
func gen_checksum(files []string, t string) bool {

	has_error := false

	for i := 0; i < len(files); i++ {
		fn := files[i]

		/* stdin */
		if fn == "-" {
			sum := calc_checksum(os.Stdin, t)
			if sum != "" {
				fmt.Fprintf(os.Stdout, "%s *%s\n", sum, fn)
			} else {
				has_error = true
			}
			continue
		}

		/* file */

		/* extends file lists when filename contains '*' */
		filenames, _ := filepath.Glob(fn)
		if filenames == nil {
			filenames = append(filenames, fn)
		}

		for _, f := range filenames {
			file, err := os.Open(f)
			if err != nil {
				has_error = true
				fmt.Fprintf(os.Stderr, "%ssum: %s\n", t, err.Error())
				continue
			}
			sum := calc_checksum(file, t)
			file.Close()
			if sum != "" {
				fmt.Fprintf(os.Stdout, "%s *%s\n", sum, f)
			} else {
				has_error = true
			}
		}
	}

	return !has_error
}

/*
   generate the checksum for given file list.

   files: the file name lists to generate checksum

   t: the type of checksum, md5 or sha1...

   return false if there are some errors.

   return true if there is no error.
*/
func GenerateChecksum(files []string, t string) bool {
	return gen_checksum(files, t)
}
