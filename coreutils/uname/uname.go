/* Go uname -- print system information

   Copyright (C) 1989-2014 Free Software Foundation, Inc.

   This program is free software: you can redistribute it and/or modify
   it under the terms of the GNU General Public License as published by
   the Free Software Foundation, either version 3 of the License, or
   (at your option) any later version.

   This program is distributed in the hope that it will be useful,
   but WITHOUT ANY WARRANTY; without even the implied warranty of
   MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
   GNU General Public License for more details.

   You should have received a copy of the GNU General Public License
   along with this program.  If not, see <http://www.gnu.org/licenses/>.  */

/*
	Written by Eric Lagergren <ericscottlagergren@gmail.com>
	Inspired by GNU coreutils and David MacKenzie <djm@gnu.ai.mit.edu>

   Some help from
   https://github.com/aisola/go-coreutils/blob/master/uname/uname.go,
   namely the syscalls
*/

// +build linux

package main

import (
	"fmt"
	flag "github.com/ogier/pflag"
	"io/ioutil"
	"os"
	"runtime"
	"strings"
	"syscall"
)

const (
	HELP = `Usage: uname [OPTION]...
Print certain system information.  With no OPTION, same as -s.

  -a, --all                print all information, in the following order,
                             except omit -p and -i if unknown:
  -s, --kernel-name        print the kernel name
  -n, --nodename           print the network node hostname
  -r, --kernel-release     print the kernel release
  -v, --kernel-version     print the kernel version
  -m, --machine            print the machine hardware name
  -p, --processor          print the processor type or "unknown"
  -i, --hardware-platform  print the hardware platform or "unknown"
  -o, --operating-system   print the operating system
      --help     display this help and exit
      --version  output version information and exit

Report uname bugs to ericscottlagergren@gmail.com
Go coreutils home page: <https://www.github.com/EricLagerg/go-coreutils/>`
	VERSION = `Go uname (Go coreutils) 1.0
Copyright (C) 2011 Free Software Foundation, Inc.
License GPLv3+: GNU GPL version 3 or later <http://gnu.org/licenses/gpl.html>.
This is free software: you are free to change and redistribute it.
There is NO WARRANTY, to the extent permitted by law.

Written by Eric Lagergren
Inspired by David MacKenzie and Michael Murphy & Abram Isola`
	GOOS     = runtime.GOOS
	GOARCH   = runtime.GOARCH
	UNKNOWN  = "unknown"
	CPU_INFO = "/proc/cpuinfo"
)

var (
	all             = flag.BoolP("all", "a", false, "print all values")
	kernelName      = flag.BoolP("kernel-name", "s", false, "Kernel system name (e.g., \"Linux\")\n")
	nodeName        = flag.BoolP("nodename", "n", false, "Name within \"some mplementation-defined network\"\n")
	release         = flag.BoolP("kernel-release", "r", false, "Operating system release (e.g. \"2.6.28\"\n")
	kernelVersion   = flag.BoolP("kernel-version", "v", false, "Kernel version")
	machine         = flag.BoolP("machine", "m", false, "Hardware name")
	processor       = flag.BoolP("processor", "p", false, "Processor type or unknown")
	hwPlatform      = flag.BoolP("hardware-platform", "i", false, "Hardware platform or unknown")
	operatingSystem = flag.BoolP("operating-system", "o", false, "Operating system")
	version         = flag.BoolP("version", "", false, "print program's version")

	name syscall.Utsname
)

type info struct {
	kname     string
	node      string
	release   string
	kversion  string
	machine   string
	processor string
	hardware  string
	os        string
}

func Proc() string {
	c, _ := ioutil.ReadFile(CPU_INFO)
	line := strings.Split(string(c), "\n")
	return string(line[4][13:])
}

func IntToString(a [65]int8) string {
	tmp := [65]byte{}
	i := 0
	for ; a[i] != 0; i++ {
		tmp[i] = uint8(a[i])
	}
	return string(tmp[:i])
}

func GenInfo() *info {
	_ = syscall.Uname(&name)
	return &info{
		kname:     IntToString(name.Sysname),
		node:      IntToString(name.Nodename),
		release:   IntToString(name.Release),
		kversion:  IntToString(name.Version),
		machine:   IntToString(name.Machine),
		processor: Proc(),
		os:        GOOS,
	}
}

func main() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "%s\n", HELP)
		return
	}
	flag.Parse()

	uname := GenInfo()

	if *version {
		fmt.Printf("%s\n", VERSION)
		return
	}
	if flag.NFlag() == 0 {
		fmt.Printf("%s\n", uname.kname)
		return
	}
	if *all {
		r := fmt.Sprintf("%s %s %s %s %s %s %s", uname.kname, uname.node, uname.release, uname.kversion, uname.machine, uname.processor, uname.os)
		fmt.Println(r)
		return
	}
	if *kernelName {
		if GOOS == "linux" {
			fmt.Printf("%s ", "Linux")
		} else {
			fmt.Printf("%s ", GOOS)
		}
	}
	if *nodeName {
		hn, err := os.Hostname()
		if err == nil {
			fmt.Printf("%s ", hn)
		}
	}
	if *release {
		fmt.Printf("%s ", uname.release)
	}
	if *kernelVersion {
		fmt.Printf("%s ", uname.kversion)
	}
	if *machine {
		fmt.Printf("%s ", uname.machine)
	}
	if *processor {
		if uname.processor == "" {
			fmt.Printf("%s ", UNKNOWN)
		} else {
			fmt.Printf("%s ", uname.processor)
		}
	}
	if *hwPlatform {
		if uname.hardware == "" {
			fmt.Printf("%s ", UNKNOWN)
		} else {
			fmt.Printf("%s ", uname.hardware)
		}
	}
	if *operatingSystem {
		if uname.os == "linux" {
			fmt.Printf("%s ", "GNU/Linux")
		} else {
			fmt.Printf("%s ", uname.os)
		}
	}
	fmt.Println()
}
