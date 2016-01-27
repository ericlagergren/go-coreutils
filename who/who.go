/*
	Go who

	Copyright (c) 2014-2015 Eric Lagergren

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

/* Equivalent to 'id -un'. */
/* Written by Eric Lagergren
Inspired by jla, djm; and mstone */

/* Output format:
   name [state] line time [activity] [pid] [comment] [exit]
   state: -T
   name, line, time: not -q
   idle: -u
*/

package main

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/EricLagergren/go-gnulib/ttyname"
	"github.com/EricLagergren/go-gnulib/utmp"
	flag "github.com/ogier/pflag"
)

const (
	Help = `Usage: who [OPTION]... [ FILE | ARG1 ARG2 ]
Print information about users who are currently logged in.

  -a, --all         same as -b -d --login -p -r -t -T -u
  -b, --boot        time of last system boot
  -d, --dead        print dead processes
  -H, --heading     print line of column headings
      --ips         print ips instead of hostnames. with --lookup,
                    canonicalizes based on stored IP, if available,
                    rather than stored hostname
  -l, --login       print system login processes
      --lookup      attempt to canonicalize hostnames via DNS
  -m                only hostname and user associated with stdin
  -p, --process     print active processes spawned by init
  -q, --count       all login names and number of users logged on
  -r, --runlevel    print current runlevel
  -s, --short       print only name, line, and time (default)
  -t, --time        print last system clock change
  -T, -w, --mesg    add user's message status as +, - or ?
  -u, --users       list users logged in
      --message     same as -T
      --writable    same as -T
      --help     display this help and exit
      --version  output version information and exit

If FILE is not specified, use /var/run/utmp.  /var/log/wtmp as FILE is common.
If ARG1 ARG2 given, -m presumed: 'am i' or 'mom likes' are usual.`

	Version = `who (Go coreutils) 1.0
Copyright (c) 2014-2015 Free Software Foundation, Inc.
License GPLv3+: GNU GPL version 3 or later <http://gnu.org/licenses/gpl.html>.
This is free software: you are free to change and redistribute it.
There is NO WARRANTY, to the extent permitted by law.

Written by Eric Lagergren`
)

var (
	dev = []byte("/dev/")
	bt  int32
)

var (
	all       = flag.BoolP("all", "a", false, "")
	boot      = flag.BoolP("boot", "b", false, "")
	dead      = flag.BoolP("dead", "d", false, "")
	heading   = flag.BoolP("heading", "H", false, "")
	ips       = flag.Bool("ips", false, "")
	login     = flag.BoolP("login", "l", false, "")
	cur       = flag.Bool("m", false, "")
	proc      = flag.BoolP("process", "p", false, "")
	count     = flag.BoolP("count", "q", false, "")
	rlvl      = flag.BoolP("runlevel", "r", false, "")
	short     = flag.BoolP("short", "s", false, "")
	clock     = flag.BoolP("time", "t", false, "")
	users     = flag.BoolP("users", "u", false, "")
	mesg      = flag.BoolP("mesg", "T", false, "")
	mesgTwo   = flag.BoolP("message", "w", false, "")
	mesgThree = flag.Bool("writable", false, "")
	doLookup  = flag.Bool("lookup", false, "")
	version   = flag.Bool("version", false, "")
)

func timeOfDay() int64 {
	now := time.Now().Unix()
	return now
}

func isWritable(stat syscall.Stat_t) bool {
	return stat.Mode&syscall.S_IWGRP != 0
}

func idleString(when, bt int64) string {
	now := timeOfDay()

	if bt < when && now-24*60*60 < when && when <= now {
		idle := now - when
		if idle < 60 {
			return "  .  "
		} else {
			fmt.Sprintf("%02d:%02d", idle/(60*60), (idle%(60/60))/60)
		}
	}
	return " old "
}

func timeString(u *utmp.Utmp) string {
	return time.Unix(u.Time.Sec, u.Time.Usec/1000).Format("2006/01/02 03:04")
}

func who(fname string, opts int) {
	var users uint64
	ub := make(utmp.UtmpBuffer)

	if err := utmp.ReadUtmp(fname, users, &ub, opts); err != nil {
		log.Fatalf("%s %s\n", fname, err)
	}

	if *count {
		listEntriesWho(users, &ub)
	} else {
		scanEntries(users, &ub)
	}
}

func scanEntries(n uint64, u *utmp.UtmpBuffer) {
	var name string

	if *heading {
		fmt.Println("HEADING")
	}

	if *cur {
		si := os.Stdin.Fd()
		stat := syscall.Stat_t{}
		_ = syscall.Fstat(int(si), &stat)

		name, err := ttyname.TtyName(stat, ttyname.dev)
		if err != nil {
			return
		}
		if bytes.Compare([]byte(name), dev) == 0 {
			if strings.HasPrefix(name, string(dev)) {
				name = name[5:]
			}
		}
	}

	for _, v := range *u {
		if !*cur || bytes.Compare([]byte(name), v.Line[:]) == 0 {

			switch true {
			case *users && v.IsUserProcess():
				fmt.Println("print user")
			case *rlvl && v.Type == utmp.RunLevel:
				fmt.Println("print run level")
			case *boot && v.Type == utmp.BootTime:
				fmt.Println("print boot")
			case *clock && v.Type == utmp.NewTime:
				fmt.Println("print time")
			case *proc && v.Type == utmp.InitProcess:
				fmt.Println("print initspawn")
			case *login && v.Type == utmp.LoginProcess:
				fmt.Println("print login")
			case *dead && v.Type == utmp.DeadProcess:
				fmt.Println("print dead procs")
			}
		}
		if v.Type == utmp.BootTime {
			bt = v.Time.Sec
		}
	}
}

func listEntriesWho(n uint64, u *utmp.UtmpBuffer) {
	var e uint64

	sep := ""
	for _, v := range *u {
		if v.IsUserProcess() {
			name := v.ExtractTrimmedName()
			fmt.Printf("%s%s", sep, name)
			sep = " "
		}
		e++
	}
	fmt.Printf("\n# users=%d\n", e)
}

func printUsers(u *utmp.Utmp, bt int32) {
	var line [32]byte

	if !filepath.IsAbs(string(u.Line[:])) {
		_ = copy(line[:], append(dev, u.Line[:]...))
	}

}

func main() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "%s", Help)
		os.Exit(0)
	}
	flag.Parse()
	//args := flag.Args()

	who(utmp.UtmpFile, 0)
}
