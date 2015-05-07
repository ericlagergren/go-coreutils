package main

import (
	"bytes"
	"fmt"
	"io"
	"math"
	"os"
	"strconv"
	"syscall"
	"time"

	"github.com/EricLagerg/go-gnulib/stdlib"
	"github.com/EricLagerg/go-gnulib/utmp"
	// "../go-gnulib/utmp"

	flag "github.com/ogier/pflag"
)

const (
	Help = `Usage:
 uptime [options]

Options:
 -p, --pretty   show uptime in pretty format
 -h, --help     display this help and exit
 -s, --since    system up since
 -V, --version  output version information and exit

For more details see uptime(1).
`

	delim    = " "
	procPath = "/proc/uptime"
)

var (
	version = flag.BoolP("version", "v", false, "")
)

func printUptime(us []utmp.Utmp) {

	var (
		bootTime int32
		entries  int64
		now      utmp.TimeVal

		days, hours, mins int
		uptime            float64
	)

	// Check if /proc/uptime exists. If not, fallback to UTMP. If that
	// doesn't work, bail. BSD implementations are similar, except use
	// sysctl instead of /proc/uptime.
	if syscall.Access(procPath, syscall.F_OK) == nil {
		file, err := os.Open(procPath)
		if err != nil {
			panic(err)
		}
		defer file.Close()

		buf := make([]byte, 256)

		n, err := file.Read(buf)
		if err != nil && err != io.EOF {
			panic(err)
		}

		// /proc/uptime's output is in the format of "%f %f\n"
		// The first space in the buffer will be the end of the first number
		line := string(buf[:bytes.IndexByte(buf[:n], ' ')])

		secs, err := strconv.ParseFloat(line, 64)
		if err != nil {
			panic(err)
		}

		if 0 <= secs || secs < math.MaxFloat64 {
			uptime = secs
		} else {
			uptime = -1
		}

	} else {
		panic("k")
	}

	for _, v := range us {

		if v.IsUserProcess() {
			fmt.Printf("%+v\n", v)
			entries++
		}

		if v.Type == utmp.BootTime {
			bootTime = v.Time.Sec
		}
	}

	now.GetTimeOfDay()
	if uptime == 0 {
		if bootTime == 0 {
			panic("can't get boot time")
		}

		uptime = float64(now.Sec - bootTime)
	}

	days = int(uptime) / 86400
	hours = (int(uptime) - (days * 86400)) / 3600
	mins = (int(uptime) - (days * 86400) - (hours * 3600)) / 60

	fmt.Print(time.Now().Local().Format(" 15:04:05 "))

	if uptime == -1 {
		fmt.Print("up ???? days ??:??,  ")
	} else {
		if 0 < days {
			if days > 1 {
				fmt.Printf("up %f days %2d:%02d,  ", days, hours, mins)
			} else {
				fmt.Printf("up %f day %2d:%02d,  ", days, hours, mins)
			}
		} else {
			fmt.Printf("up %2d:%02d,  ", hours, mins)
		}
	}

	if len(us) > 1 || len(us) == 0 {
		fmt.Printf("%d users", len(us))
	} else {
		fmt.Printf("%d user", len(us))
	}

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
	us := make([]utmp.Utmp, 0)
	err := utmp.ReadUtmp(fname, &entries, &us, opts)
	if err != nil {
		panic(err)
	}

	printUptime(us)
}

func main() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "%s", Help)
		os.Exit(1)
	}
	flag.Parse()

	if *version {
		fmt.Printf("%s\n", "k")
		os.Exit(0)
	}

	uptime(utmp.UtmpFile, 0)
}
