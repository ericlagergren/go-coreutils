// Copyright (c) 2014-2016 Eric Lagergren
// Use of this source code is governed by the GPL v3 or later.

package main

import (
	"fmt"
	"log"
	"os"

	"github.com/EricLagergren/go-coreutils/internal/flag"
)

const (
	unknown = "unknown"
	procCPU = "/proc/cpuinfo"
	maxUint = ^printer(0)
)

var (
	all = flag.BoolP("all", "a", false, `print all information, in the following order,
                             except omit -p and -i if unknown:`)
	kernelName      = flag.BoolP("kernel-name", "s", false, "print the kernel name")
	nodeName        = flag.BoolP("nodename", "n", false, "print the network node hostname")
	release         = flag.BoolP("kernel-release", "r", false, "print the kernel release")
	version         = flag.BoolP("kernel-version", "v", false, "print the kernel version")
	machine         = flag.BoolP("machine", "m", false, "print the machine hardware name")
	processor       = flag.BoolP("processor", "p", false, "print the processor tyoe or \"unknown\"")
	hwPlatform      = flag.BoolP("hardware-platform", "i", false, "print the hardware platform or \"unknown\"")
	operatingSystem = flag.BoolP("operating-system", "o", false, "print the operating system")

	fatal = log.New(os.Stderr, "", 0)
)

type printer uint

// Enumerated printing options.
const (
	printKernelName printer = 1 << iota
	printNodeName
	printKernelRelease
	printKernelVersion
	printMachine
	printProcessor
	printHardwarePlatform
	printOperatingSystem
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
	toprint |= decode(*all, maxUint)
	toprint |= decode(*kernelName, printKernelName)
	toprint |= decode(*nodeName, printNodeName)
	toprint |= decode(*release, printKernelRelease)
	toprint |= decode(*version, printKernelVersion)
	toprint |= decode(*machine, printMachine)
	toprint |= decode(*processor, printProcessor)
	toprint |= decode(*hwPlatform, printHardwarePlatform)
	toprint |= decode(*operatingSystem, printOperatingSystem)
	return toprint
}

func main() {
	flag.Usage = func() {
		fmt.Printf(`Usage: %s [OPTION]...
Print certain system information. With no OPTION, same as -s.

`, flag.Program)
		flag.DBE()
	}
	flag.ProgVersion = "1.1"
	flag.Parse()

	toprint := decodeFlags()
	if toprint == 0 {
		toprint = printKernelName
	}

	if toprint.isSet(
		(printKernelName | printNodeName | printKernelRelease |
			printKernelVersion | printMachine)) {

		name, err := genInfo()
		if err != nil {
			fatal.Fatalln("cannot get system name")
		}

		if toprint.isSet(printKernelName) {
			print(name.sysname)
		}
		if toprint.isSet(printNodeName) {
			print(name.nodename)
		}
		if toprint.isSet(printKernelRelease) {
			print(name.release)
		}
		if toprint.isSet(printKernelVersion) {
			print(name.version)
		}
		if toprint.isSet(printMachine) {
			print(name.machine)
		}
	}

	if toprint.isSet(printProcessor) {
		element := unknown
		if toprint != maxUint || element != unknown {
			print(element)
		}
	}

	if toprint.isSet(printHardwarePlatform) {
		element := unknown
		if toprint != maxUint || hostOS != unknown {
			print(element)
		}
	}

	if toprint.isSet(printOperatingSystem) {
		print(hostOS)
	}

	os.Stdout.WriteString("\n")
}
