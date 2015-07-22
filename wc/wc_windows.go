/*
	Go wc - print the lines, words, bytes, and characters in files

	Copyright (C) 2015 Eric Lagergren

	This program is free software: you can redistribute it and/or modify
	it under the terms of the GNU General Public License as published by
	the Free Software Foundation, either version 3 of the License, or
	(at your option) any later version.

	This program is distributed in the hope that it will be useful,
	but WITHOUT ANY WARRANTY; without even the implied warranty of
	MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
	GNU General Public License for more details.

	You should have received a copy of the GNU General Public License
	along with this program.  If not, see <http://www.gnu.org/licenses/>.
*/

/*
	Written by Eric Lagergren <ericscottlagergren@gmail.com>
	Inspired by GNU's wc, which was written by
	Paul Rubin, phr@ocf.berkeley.edu and David MacKenzie, djm@gnu.ai.mit.edu
*/

package main

import (
	"io"
	"os"
	"unicode"
	"unicode/utf8"
)

// string of file name, file offset, and fstatus struct
func wc(file *os.File, cur int64, status *fstatus) int {
	// Our temp number of lines, words, chars, and bytes
	var (
		lines      int64
		words      int64
		chars      int64
		numBytes   int64
		lineLength int64
		linePos    int64
		inWord     int64

		buffer = make([]byte, BufferSize)
		// return value
		ok = 0
	)

	countComplicated := *printWords || *printLineLength

	/* Normally we'd call fadvise() here but Windows doesn't *really*
	support that... */

	// If we simply want the bytes we can ignore the overhead (see: GNU
	// wc.c by Paul Rubin and David MacKenzie) of counting lines, chars,
	// and words
	if *printBytes && !*printChars && !*printLines && !countComplicated {

		// Manually count bytes if Stat() failed or if we're reading from
		// piped input (e.g. cat file.csv | wc -c -)
		if status.stat == nil || status.stat.Mode()&os.ModeNamedPipe != 0 {
			for {
				n, err := file.Read(buffer)
				if err != nil && err != io.EOF {
					ok = 1
					break
				}

				numBytes += int64(n)

				if err == io.EOF {
					break
				}
			}
		} else {
			numBytes = status.stat.Size()
			end := numBytes
			high := end - end%BufferSize
			if cur <= 0 {
				cur, _ = file.Seek(0, os.SEEK_CUR)
			}
			if 0 <= cur && cur < high {
				if n, _ := file.Seek(high, os.SEEK_CUR); 0 <= n {
					numBytes = high - cur
				}
			}
		}

		// Use a different loop to lower overhead if we're *only* counting
		// lines (or lines and bytes)
	} else if !*printChars && !countComplicated {
		for {
			n, err := file.Read(buffer)
			if err != nil && err != io.EOF {
				ok = 1
				break
			}

			lines += count(buffer[:n], NewLineByte)
			numBytes += int64(n)

			if err == io.EOF {
				break
			}
		}
	} else {
		for {
			n, err := file.Read(buffer)
			numBytes += int64(n)

			b := buffer[:n]

			for len(b) > 0 {
				r, s := utf8.DecodeRune(b)

				switch r {
				case NewLine:
					lines++
					fallthrough
				case Return:
					fallthrough
				case FormFeed:
					if linePos > lineLength {
						lineLength = linePos
					}
					linePos = 0
					words += inWord
					inWord = 0
				case HorizTab:
					linePos += *tabWidth - (linePos % *tabWidth)
					words += inWord
					inWord = 0
				case Space:
					linePos++
					fallthrough
				case VertTab:
					words += inWord
					inWord = 0
				default:
					if unicode.IsPrint(r) {
						linePos++
						inWord = 1
					}
				}

				chars++
				b = b[s:]
			}

			if err == io.EOF {
				break
			} else if err != nil {
				ok = 1
				break
			}
		}
		if linePos > lineLength {
			lineLength = linePos
		}

		words += inWord
	}

	writeCounts(lines, words, chars, numBytes, lineLength, file.Name())

	totalBytes += numBytes
	totalChars += chars
	totalLines += lines
	totalWords += words

	if lineLength > maxLineLength {
		maxLineLength = lineLength
	}

	return ok
}
