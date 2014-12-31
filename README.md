### Go coreutils

This is a port of GNU's coreutils (http://www.gnu.org/software/coreutils/).

**It's currently under development.**

### Completed:

|Utility|Completeness|
|:_______-------|:------------|
|wc     |100%        |
|uname  |100%        |
|cat    |100% (bugs?)|
|chown  |100% (buggy -R)|
|whoami |100%        |
|tty    |100%        |

These utilities should be nearly identical to GNU's coreutils, and should have *relatively* the same speed. For example, wc.go counts chars in 550MB file in < 15sec, wc.c in ~11sec on (Intel core i3 2.66ghz running Debian 3.2.63-2+deb7u1 x86_64).

It's licensed under the GPLv3 because it's an attempted direct port from GNU's coreutils, which are licensed under GPLv3, using BSD-licensed code (Go's own source).

### REQUIRES "github.com/ogier/pflag"
You can get `pflag` through `go get github.com/ogier/pflag`

### LICENSE:

```
   Copyright (C) 2014 Eric Lagergren

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
```
