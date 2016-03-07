// Copyright (c) 2015 Eric Lagergren
// Use of this source code is governed by the GPL v3 or later.

package main

import (
	"bytes"
	"io"
	"os"
	"unicode"
	"unicode/utf8"
)

// string of file name, file offset, and fstatus struct
func wc(file *os.File, cur int64, status fstatus) int {
	// Our temp number of lines, words, chars, and bytes
	var (
		lines      int64
		words      int64
		chars      int64
		numBytes   int64
		lineLength int64

		buffer [bufferSize + 1]byte

		// Return value.
		ok = 0
	)

	countComplicated := *printWords || *printLineLength

	// Normally we'd call fadvise() here but Windows doesn't *really*
	// support that...

	if *printBytes && !*printChars && !*printLines && !countComplicated {

		// Manually count bytes if Stat() failed or if we're reading from
		// piped input (e.g. cat file.csv | wc -c -)
		if status.failed != nil {
			status.stat, status.failed = file.Stat()
		}

		// For sized files, seek a block from EOF.
		// From GNU's source:
		//
		// "This works better for files in proc-like file systems where
		// the size is only approximate."
		if status.failed == nil &&
			// Regular file but not stdin.
			((status.stat.Mode().IsRegular() &&
				status.stat.Mode()&os.ModeCharDevice != 0) ||
				status.stat.Mode()&os.ModeSymlink != 0) &&
			0 < status.stat.Size() {

			numBytes = status.stat.Size()
			end := numBytes
			high := end - end%(blockSize+1)
			if cur < 0 {
				cur, _ = file.Seek(0, os.SEEK_CUR)
			}
			if 0 <= cur && cur < high {
				if n, _ := file.Seek(high, os.SEEK_CUR); 0 <= n {
					numBytes = high - cur
				}
			}
		}

		for {
			n, err := file.Read(buffer[:])
			if err != nil {
				if err != io.EOF {
					ok = 1
				}
				break
			}
			numBytes += int64(n)
		}

		// Use a different loop to lower overhead if we're *only* counting
		// lines (or lines and bytes)
	} else if !*printChars && !countComplicated {
		for {
			n, err := file.Read(buffer[:])
			if err != nil {
				if err != io.EOF {
					ok = 1
				}
				break
			}

			// Go doesn't inline this sooo...
			for i := 0; i < n; i++ {
				if buffer[i] != '\n' {
					o := bytes.IndexByte(buffer[i:n], '\n')
					if o < 0 {
						break
					}
					i += o
				}
				lines++
			}

			numBytes += int64(n)
		}
	} else {
		var (
			inWord  int64
			linePos int64
		)
		for {
			n, err := file.Read(buffer[:])
			if err != nil {
				if err != io.EOF {
					ok = 1
				}
				break
			}

			numBytes += int64(n)

			for bp := 0; bp < n; {
				r, s := utf8.DecodeRune(buffer[bp:])

				switch r {
				case '\n':
					lines++
					fallthrough
				case '\r':
					fallthrough
				case '\f':
					if linePos > lineLength {
						lineLength = linePos
					}
					linePos = 0
					words += inWord
					inWord = 0
				case '\t':
					linePos += *tabWidth - (linePos % *tabWidth)
					words += inWord
					inWord = 0
				case ' ':
					linePos++
					fallthrough
				case '\v':
					words += inWord
					inWord = 0
				default:
					if unicode.IsPrint(r) {
						linePos++
						if unicode.IsSpace(r) {
							words += inWord
							inWord = 0
						}
						inWord = 1
					}
				}

				chars++
				bp += s
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
