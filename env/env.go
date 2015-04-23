/*
	Go env - run a program in a modified environment
	Copyright (C) 1986-2015 Eric Lagergren

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
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"

	flag "github.com/ogier/pflag"
)

const (
	Help = `
	Usage: env [OPTION]... [-] [NAME=VALUE]... [COMMAND [ARG]...]
Set each NAME to VALUE in the environment and run COMMAND.

Mandatory arguments to long options are mandatory for short options too.
  -i, --ignore-environment  start with an empty environment
  -0, --null           end each output line with NUL, not newline
  -u, --unset=NAME     remove variable from the environment
  -s, --set=NAME       set variable in the environment
      --help           display this help and exit
      --version        output version information and exit

A mere - implies -i.  If no COMMAND, print the resulting environment.

Report wc bugs to ericscottlagergren@gmail.com
Go coreutils home page: <https://www.github.com/EricLagerg/go-coreutils/>
`
	Version = `env (Go coreutils) 1.0
Copyright (C) 2015 Eric Lagergren
License GPLv3+: GNU GPL version 3 or later <http://gnu.org/licenses/gpl.html>.
This is free software: you are free to change and redistribute it.
There is NO WARRANTY, to the extent permitted by law.
`
)

const delim = "="

var (
	unset   = flag.StringP("unset", "u", "", "")
	nullEol = flag.BoolP("null", "0", false, "")
	ignore  = flag.BoolP("ignore-environment", "i", false, "")
	version = flag.BoolP("version", "v", false, "")

	fatal = log.New(os.Stderr, "", log.Lshortfile)
	// fatal = log.New(os.Stderr, "", 0)

	env = os.Environ()
)

// Run a command, waiting for it to finish. Will first run CMD's path;
// failing that, will lookup the path and attempt to do the same.
func execvp(cmd *exec.Cmd) error {
	if err := cmd.Start(); err == nil {

		// Wait for command to finish
		return cmd.Wait()

	} else {

		// Didn't work? Search for the executable's path
		path, err := exec.LookPath(cmd.Path)
		if err != nil {
			return err
		}

		// Reset our path
		cmd.Path = path

		// Try again with the executable found in $PATH
		err = cmd.Start()
		if err != nil {
			return err
		}

		return cmd.Wait()
	}
}

func main() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "%s", Help)
		os.Exit(1)
	}
	flag.Parse()

	if *version {
		fmt.Printf("%s", Version)
		os.Exit(0)
	}

	if *unset != "" {
		os.Unsetenv(*unset)
	}

	cmd := new(exec.Cmd)
	cmd.Env = env

	// Check for "-" as an argument, because it means the same as "-i"
	if flag.Arg(0) == "-" {
		cmd.Env = make([]string, 0)
	}

	for i, arg := range flag.Args() {
		if strings.Index(arg, delim) > 0 {
			cmd.Env = append(cmd.Env, arg)
		} else if arg != "-" {
			if *nullEol {
				fatal.Fatalln("cannot specify --null (-0) with command")
			}

			cmd.Path = arg

			cmd.Args = append(cmd.Args, flag.Args()[i:]...)
			cmd.Stdin = os.Stdin
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr

			err := execvp(cmd)
			if err != nil {
				fatal.Fatalln(err)
			}
			return
		}

		i++
	}

	eol := '\n'
	if *nullEol {
		eol = '\x00'
	}

	for _, e := range env {
		fmt.Printf("%s%c", e, eol)
	}

	return
}
