/*
   Go uname -- print system information

   Copyright (c) 2014-2015  Eric Lagergren

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

// Written by Eric Lagergren <ericscottlagergren@gmail.com>

package main

import (
	"fmt"
	"log"
	"os"

	flag "github.com/ogier/pflag"
)

const (
	Help = `Usage: uname [OPTION]...
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
	Version = `Go uname (Go coreutils) 1.0
Copyright (C) 2015 Eric Lagergren.
License GPLv3+: GNU GPL version 3 or later <http://gnu.org/licenses/gpl.html>.
This is free software: you are free to change and redistribute it.
There is NO WARRANTY, to the extent permitted by law.

Written by Eric Lagergren`

	Unknown = "unknown"
	ProcCPU = "/proc/cpuinfo"
	MaxUint = ^printer(0)
)

var (
	all             = flag.BoolP("all", "a", false, "")
	kernelName      = flag.BoolP("kernel-name", "s", false, "")
	nodeName        = flag.BoolP("nodename", "n", false, "")
	release         = flag.BoolP("kernel-release", "r", false, "")
	kernelVersion   = flag.BoolP("kernel-version", "v", false, "")
	machine         = flag.BoolP("machine", "m", false, "")
	processor       = flag.BoolP("processor", "p", false, "")
	hwPlatform      = flag.BoolP("hardware-platform", "i", false, "")
	operatingSystem = flag.BoolP("operating-system", "o", false, "")
	version         = flag.BoolP("version", "", false, "")

	fatal = log.New(os.Stderr, "", 0)
)

type printer uint

// Enumerated printing options.
const (
	PrintKernelName printer = 1 << iota
	PrintNodeName
	PrintKernelRelease
	PrintKernelVersion
	PrintMachine
	PrintProcessor
	PrintHardwarePlatform
	PrintOperatingSystem
)

func (p printer) isSet(val printer) bool {
	return p&val != 0
}

var printed bool

func print(element string) {
	if printed {
		fmt.Print(" ")
	}
	printed = true
	fmt.Printf("%s", element)
}

func decode(b bool, flag printer) printer {
	if b {
		return flag
	}
	return 0
}

func decodeFlags() printer {
	var toprint printer

	toprint |= decode(*all, MaxUint)
	toprint |= decode(*kernelName, PrintKernelName)
	toprint |= decode(*nodeName, PrintNodeName)
	toprint |= decode(*release, PrintKernelRelease)
	toprint |= decode(*version, PrintKernelVersion)
	toprint |= decode(*machine, PrintMachine)
	toprint |= decode(*processor, PrintProcessor)
	toprint |= decode(*hwPlatform, PrintHardwarePlatform)
	toprint |= decode(*operatingSystem, PrintOperatingSystem)

	return toprint
}

// WHY OH WHY OH WHY DO I HAVE TO USE GO GENERATE.

//go:generate ostypes main
func main() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "%s\n", Help)
		return
	}
	flag.Parse()

	if *version {
		fmt.Printf("%s\n", Version)
		return
	}

	toprint := decodeFlags()

	if toprint == 0 {
		toprint = PrintKernelName
	}

	if toprint.isSet(
		(PrintKernelName | PrintNodeName | PrintKernelRelease |
			PrintKernelVersion | PrintMachine)) {

		name, err := genInfo()
		if err != nil {
			fatal.Fatalln("cannot get system name")
		}

		if toprint.isSet(PrintKernelName) {
			print(name.sysname)
		}
		if toprint.isSet(PrintNodeName) {
			print(name.nodename)
		}
		if toprint.isSet(PrintKernelRelease) {
			print(name.release)
		}
		if toprint.isSet(PrintKernelVersion) {
			print(name.version)
		}
		if toprint.isSet(PrintMachine) {
			print(name.machine)
		}
	}

	if toprint.isSet(PrintProcessor) {
		element := Unknown
		if !(toprint == MaxUint && element == Unknown) {
			print(element)
		}
	}

	if toprint.isSet(PrintHardwarePlatform) {
		element := HostOS
		if !(toprint == MaxUint && element == Unknown) {
			print(element)
		}
	}

	fmt.Println()
}
