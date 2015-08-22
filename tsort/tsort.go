/*
	Go tsort - topological sort

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

package main

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"os"
	"strings"

	"golang.org/x/sys/unix"

	flag "github.com/ogier/pflag"
)

const (
	Help = `Usage: tsort [OPTION] [FILE]
Write totally ordered list consistent with the partial ordering in FILE.
With no FILE, or when FILE is -, read standard input.

      --help     display this help and exit
      --version  output version information and exit
Report wc bugs to ericscottlagergren@gmail.com
Go coreutils home page: <https://www.github.com/EricLagerg/go-coreutils/>
`

	Version = `tsort (Go coreutils) 1
Copyright (C) 2015 Eric Lagergren.
License GPLv3+: GNU GPL version 3 or later <http://gnu.org/licenses/gpl.html>.
This is free software: you are free to change and redistribute it.
There is NO WARRANTY, to the extent permitted by law.
`
)

var (
	version = flag.BoolP("version", "v", false, "")

	fatal = log.New(os.Stderr, "", 0)
)

type successor struct {
	suc  *item
	next *successor
}

type item struct {
	str     string
	left    *item
	right   *item
	balance int // -1, 0, or +1
	count   int64
	qlink   *item
	top     *successor
}

type action func(*item) bool

var (
	head  *item // Head of list
	zeros *item // Tail of the list of 'zeros' (no predecessors)
	loop  *item // Used for loop detection

	numStrings int64 // Strings to sort
)

func newItem(str string) *item { return &item{str: str} }

// Search binary tree for STR. If ROOT is nil, create a new tree.
func (root *item) searchItem(str string) *item {
	var (
		p = new(item)
		q = new(item)
		r = new(item)
		s = new(item)
		t = new(item)
	)

	if root.right == nil {
		root.right = newItem(str)
		return root.right
	}

	t = root
	p = root.right
	s = p

	for {
		a := strings.Compare(str, p.str)
		if a == 0 {
			return p
		}

		if a < 0 {
			q = p.left
		} else {
			q = p.right
		}

		if q == nil {
			q = newItem(str)

			if a < 0 {
				p.left = q
			} else {
				p.right = q
			}

			if strings.Compare(str, s.str) < 0 {
				p = s.left
				r = p
				a = -1
			} else {
				p = s.right
				r = p
				a = 1
			}

			for p != q {
				if strings.Compare(str, p.str) < 0 {
					p.balance = -1
					p = p.left
				} else {
					p.balance = 1
					p = p.right
				}
			}

			if s.balance == 0 || s.balance == -a {
				s.balance += a
				return q
			}

			if r.balance == a {

				p = r
				if a < 0 {
					s.left = r.right
					r.right = s
				} else {
					s.right = r.left
					r.left = s
				}
				r.balance = 0
				s.balance = r.balance
			} else {
				if a < 0 {
					p = r.right
					r.right = p.left
					p.left = r
					s.left = p.right
					p.right = s
				} else {
					p = r.left
					r.left = p.right
					p.right = r
					s.right = p.left
					p.left = s
				}

				s.balance = 0
				r.balance = 0

				if p.balance == a {
					s.balance = -a
				} else if p.balance == -a {
					r.balance = a
				}

				p.balance = 0
			}

			if s == t.right {
				t.right = p
			} else {
				t.left = p
			}

			return q

		}

		if q.balance > 0 {
			t = p
			s = q
		}

		p = q
	}
}

func recordRelation(j, k *item) {
	if j.str != k.str {
		k.count++
		p := &successor{
			suc:  k,
			next: j.top,
		}
		j.top = p
	}
}

func countItems(_ *item) bool {
	numStrings++
	return false
}

func scanZeros(k *item) bool {
	if k.count == 0 && k.str != "" {
		if head == nil {
			head = k
		} else {
			zeros.qlink = k
		}

		zeros = k
	}

	return false
}

// Try and detect loops. e.g.,
// 1 2
// 2 1
// If any are found, print to stderr.
func detectLoop(k *item) bool {
	if k.count > 0 {
		if loop == nil {
			loop = k
		} else {
			p := &k.top

			for *p != nil {
				if (*p).suc == loop {
					if k.qlink != nil {

						for loop != nil {
							tmp := loop.qlink

							fatal.Printf("tsort: %s", loop.str)

							if loop == k {
								(*p).suc.count--
								*p = (*p).next
								break
							}

							loop.qlink = nil
							loop = tmp
						}

						for loop != nil {
							tmp := loop.qlink

							loop.qlink = nil
							loop = tmp
						}

						return true
					}

					k.qlink = loop
					loop = k
					break
				}
				p = &(*p).next
			}
		}
	}

	return false
}

// Recurse the tree at ROOT, calling ACTION for each node
func (root *item) recurseTree(fn action) bool {
	if root.left == nil && root.right == nil {
		return fn(root)
	}

	if root.left != nil {
		if root.left.recurseTree(fn) {
			return true
		}
	}

	if fn(root) {
		return true
	}

	if root.right != nil {
		if root.right.recurseTree(fn) {
			return true
		}
	}

	return false
}

// Walk the tree at ROOT, calling ACTION for each node
func (root *item) walkTree(fn action) {
	if root.right != nil {
		root.right.recurseTree(fn)
	}
}

func tsort(rw io.ReadWriter) int {

	var (
		root = newItem("")

		j *item
		k *item

		ok int
	)

	scanner := bufio.NewScanner(rw)
	scanner.Split(bufio.ScanWords)

	// https://talks.golang.org/2015/tricks.slide#16
	if file, ok := rw.(interface {
		Fd() uintptr
	}); ok {
		unix.Fadvise(int(file.Fd()), 0, 0, unix.FADV_SEQUENTIAL)
	}

	for scanner.Scan() {
		k = root.searchItem(scanner.Text())

		if j != nil {
			recordRelation(j, k)
			k = nil
		}

		j = k
	}

	if k != nil {
		fatal.Fatalln("input contains an odd number of tokens")
	}

	root.walkTree(countItems)

	for numStrings > 0 {

		root.walkTree(scanZeros)

		for head != nil {
			p := head.top

			fmt.Fprintln(rw, head.str)

			head.str = ""
			numStrings--

			for p != nil {
				p.suc.count--
				if p.suc.count == 0 {
					zeros.qlink = p.suc
					zeros = p.suc
				}

				p = p.next
			}

			head = head.qlink
		}

		if numStrings > 0 {
			fatal.Print("tsort: input contains a loop:")
			ok = 1

			for {
				root.walkTree(detectLoop)

				if loop == nil {
					break
				}
			}
		}
	}

	return ok
}

func main() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "%s", Help)
		os.Exit(1)
	}
	flag.Parse()

	if *version {
		fmt.Printf("%s\n", Version)
		os.Exit(0)
	}

	if flag.NArg() > 1 {
		fatal.Fatalf("extra operand %s", flag.Arg(1))
	}

	file := os.Stdin
	if (flag.Arg(0) != "-" &&
		flag.Arg(0) != "") ||
		flag.NArg() != 0 {

		var err error
		file, err = os.Open(flag.Arg(0))
		if err != nil {
			fatal.Fatalln(err)
		}
		defer file.Close()
	}

	os.Exit(tsort(struct {
		io.Reader
		io.Writer
	}{
		file,
		os.Stdout,
	}))
}
