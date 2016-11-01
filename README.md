# Go coreutils

[![forthebadge](http://forthebadge.com/images/badges/made-with-crayons.svg)](http://forthebadge.com)
[![forthebadge](http://forthebadge.com/images/badges/as-seen-on-tv.svg)](http://forthebadge.com)

This is a port of GNU's coreutils (http://www.gnu.org/software/coreutils/)
that aims to be a drop-in, cross-platform replacement.

**It's currently under development.**

Because it imports from `github.com/EricLagergren/go-gnulib`, and I'm constantly
refactoring, parts could break from day-to-day.

I'd recommend running `go get -u ...` before you file a bug report!

*Pull requests are more than welcome.*

Also, see https://www.github.com/EricLagergren/go-gnulib for a similar project that this project depends on.

### Completed:

* 100% Completion
 * 15/100
* Partial Completion
 * 2/100

| Utility | Completeness   | Cross Platform      | Need Refactor|
|:--------|:---------------|:--------------------|:-------------|
| cat     | 100%           | Yes (Unix/Windows)  | No           |
| chown   | 0% (* see note #1) | No             | Yes (-R)     |
| env     | 100%           | Yes (Unix/Windows)  | No           |
| false   | 100%           | Yes (Unix/Windows)  | No           |
| logname | 100%           | No                  | No           |
| pwd     | 100%           | Yes (Unknown)       | No           |
| sync    | 100%           | Yes (Unix/Windows)  | No           |
| true    | 100%           | Yes (Unix/Windows)  | No           |
| tsort   | 100%           | Yes (Unix/Windows)  | No           |
| tty     | 100%           | Yes (Unix/Windows)  | No           |
| uname   | 100%           | No                  | No           |
| uptime  | 90%            | Yes (Unix/Windows, no FreeBSD)  | No           |
| users   | 100%           | No                  | No           |
| wc      | 100%           | Yes (Unix/Windows)  | No           |
| whoami  | 100%           | Yes (Unix/Windows   | No           |
| xxd     | 100%           | Yes (Unix/Windows)  | No           |
| yes     | 100%           | Yes (Unix/Windows)  | No           |

* chown note: Currently refactoring from the ground-up.

**Side notes:**
- Unix *should* include OS X unless otherwise specified.
- Gofmt means it needs its styling changes (e.g. variable names, formatting, etc.)
- Idiomatic means it needs to be changed to more idiomatic Go
- Windows coverage will increase when I get a Windows laptop

### Information:

#### Performance:

Obviously there's some things Go can do better (parallelism and concurrency),
but for the most part these tools should have nearly the same speed,
with Go being slightly slower.

```
eric@archbox $ time ./wc_go -lwmc one_gigabyte_file.txt 
  32386258  146084896 1182425560 1183778772 one_gigabyte_file.txt

real  0m25.206s
user  0m24.900s
sys   0m0.313s
eric@archbox $ time wc_c -lwmc one_gigabyte_file.txt 
  32386258  146084896 1182425560 1183778772 one_gigabyte_file.txt

real  0m22.841s
user  0m22.570s
sys   0m0.257s
```

#### Behavior:

These utilities should be nearly identical to GNU's coreutils.

Since parsing the output of shell commands isn't uncommon (even if
it *is* bad behavior), most of the commands should have output that
is nearly identical to the original GNU commands.

Do note that sometimes the results could differ a little for select commands.

For example, GNU's `wc` utility relies on the current locale to determine
whether it should parse multi-byte characters or not.

The Go version, on the other hand, uses the `unicode/utf8` package
which natively detects multi-byte sequences. The trade-off is this: the
Go version is technically more correct, while the C version is faster.

Our implementation of `xxd` is actually much faster than the native `xxd`
implementation found on most *nix machines -- try it out!

### REQUIRES:

(Depends on platform and command...)
- go get github.com/EricLagergren/ostypes
- go get golang.org/x/sys/unix
- go get github.com/EricLagergren/go-gnulib/ttyname
- go get github.com/EricLagergren/go-gnulib/sysinfo
- go get github.com/EricLagergren/go-gnulib/posix
- go get github.com/EricLagergren/go-gnulib/general
- go get github.com/EricLagergren/go-gnulib/login

### LICENSE:

```
   Copyright (C) 2014-2016 Eric Lagergren

   This program is free software: you can redistribute it and/or modify
   it under the terms of the GNU General Public License as published by
   the Free Software Foundation, either version 3 of the License, or
   (at your option) any later version.

   This program is distributed in the hope that it will be useful,
   but WITHOUT ANY WARRANTY; without even the implied warranty of
   MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
   GNU General Public License for more details.

   You should have received a copy of the GNU General Public License
   along with this program.  If not, see <https://www.gnu.org/licenses/>.
```

#### License subnotes:
It (as a whole) is licensed under the GPLv3 because it's mostly a
transliteration of GNU's coreutils, which are licensed under the GPLv3.

However, certain parts have their own licenses (e.g., `xxd` is public domain).
