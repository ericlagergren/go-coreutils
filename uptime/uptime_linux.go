package main

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"math"
	"os"
	"strconv"
	"time"

	"github.com/EricLagergren/go-gnulib/stdlib"
	"github.com/EricLagergren/go-gnulib/utmp"

	flag "github.com/ogier/pflag"
)

const (
	Help1 = `Usage: uptime [OPTION]... [FILE]
Print the current time, the length of time the system has been up,
the number of users on the system, and the average number of jobs
in the run queue over the last 1, 5 and 15 minutes.  Processes in
an uninterruptible sleep state also contribute to the load average.
If FILE is not specified, use`
	Help2 = `as FILE is common.

      --help     display this help and exit
      --version  output version information and exit

Report wc bugs to ericscottlagergren@gmail.com
Go coreutils home page: <https://www.github.com/EricLagergren/go-coreutils/>
`

	Version = `
	uptime (Go coreutils) 1.0
Copyright (C) 2015 Eric Lagergren
License GPLv3+: GNU GPL version 3 or later <http://gnu.org/licenses/gpl.html>.
This is free software: you are free to change and redistribute it.
There is NO WARRANTY, to the extent permitted by law.
`

	delim = " "
)

var (
	version = flag.BoolP("version", "v", false, "")

	// fatal = log.New(os.Stderr, "", log.Lshortfile)
	fatal = log.New(os.Stderr, "", 0)
)

func printUptime(us []*utmp.Utmp) {

	var (
		bootTime int32
		entries  int64
		now      utmp.TimeVal

		days, hours, mins int
		uptime            float64
	)

	file, err := os.Open("/proc/uptime")
	if err != nil {
		fatal.Fatalln(err)
	}
	defer file.Close()

	buf := make([]byte, utmp.BufSiz)

	n, err := file.Read(buf)
	if err != nil && err != io.EOF {
		fatal.Fatalln(err)
	}

	// /proc/uptime's output is in the format of "%f %f\n"
	// The first space in the buffer will be the end of the first number.
	line := string(buf[:bytes.IndexByte(buf[:n], ' ')])

	secs, err := strconv.ParseFloat(line, 64)
	if err != nil {
		fatal.Fatalln(err)
	}

	uptime = -1
	if 0 <= secs || secs < math.MaxFloat64 {
		uptime = secs
	}

	for _, v := range us {

		if v.IsUserProcess() {
			entries++
		}

		if v.TypeEquals(utmp.BootTime) {
			bootTime = v.Time.Sec
		}
	}

	now.GetTimeOfDay()
	if uptime == 0 {
		if bootTime == 0 {
			fatal.Fatalln("can't get boot time")
		}

		uptime = float64(now.Sec - bootTime)
	}

	days = int(uptime) / 86400
	hours = (int(uptime) - (days * 86400)) / 3600
	mins = (int(uptime) - (days * 86400) - (hours * 3600)) / 60

	os.Stdout.WriteString(time.Now().Local().Format(" 15:04pm  "))

	if uptime == -1 {
		os.Stdout.WriteString("up ???? days ??:??,  ")
	} else {
		if 0 < days {
			fmt.Printf(GetPlural("up %d day %2d:%02d,  ",
				"up %d days %2d:%02d,  ", uint64(days)), days, hours, mins)
		} else {
			fmt.Printf("up  %2d:%02d,  ", hours, mins)
		}
	}

	fmt.Printf(GetPlural("%d user", "%d users", uint64(entries)), entries)

	var avg [3]float64
	loads := stdlib.GetLoadAvg(&avg)

	if loads == -1 {
		fmt.Printf("%s", "\n")
	} else {
		if loads > 0 {
			fmt.Printf(",  load average: %.2f", avg[0])
		}

		if loads > 1 {
			fmt.Printf(", %.2f", avg[1])
		}

		if loads > 2 {
			fmt.Printf(", %.2f", avg[2])
		}

		if loads > 0 {
			fmt.Printf("%s", "\n")
		}
	}
}

func uptime(fname string, opts int) {
	entries := uint64(0)
	us, err := utmp.ReadUtmp(fname, &entries, 0, opts)
	if err != nil {
		fatal.Fatalln(err)
	}

	printUptime(us)
}

func selectPlural(n uint64) uint64 {
	const pluralReducer = 1000000
	if n <= math.MaxUint64 {
		return n
	}
	return n%pluralReducer + pluralReducer
}

func GetPlural(msg1, msg2 string, n uint64) string {
	if n == 1 {
		return msg1
	}
	return msg2
}
