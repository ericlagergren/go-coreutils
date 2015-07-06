/*
	Go pwd - prints the current working directory.
	Copyright (C) 2015 Robert Deusser
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
	Written by Robert Deusser <iamthemuffinman@outlook.com>
*/

package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/codegangsta/cli"
)

func main() {
	app := cli.NewApp()
	app.Name = "pwd"
	app.Version = "0.0.1"
	app.Usage = "print name of current/working directory"
	app.Action = func(c *cli.Context) {
		dir, err := filepath.Abs(filepath.Dir(os.Args[0]))
		if err != nil {
			log.Fatal(err)
		}
		fmt.Println(dir)
	}
	app.Commands = []cli.Command{
		{
			Name:    "-L",
			Aliases: []string{"--logical"},
			Usage:   "use PWD from environment, even if it contains symlinks",
			Action: func(c *cli.Context) {
				// TODO
			},
		},
		{
			Name:    "-P",
			Aliases: []string{"--physical"},
			Usage:   "avoid all symlinks",
			Action: func(c *cli.Context) {
				// TODO
			},
		},
	}
	app.Run(os.Args)
}
