### Go coreutils

This is a port of GNU's coreutils (http://www.gnu.org/software/coreutils/).

**It's currently under development.**

These utilities should be nearly identical to GNU's coreutils, and should have *relatively* the same speed. For example, wc.go counts chars in 550MB file in < 15sec, wc.c in ~11sec on (Intel core i3 2.66ghz running Debian 3.2.63-2+deb7u1 x86_64).

It's licensed under the GPLv3 because it's an attempted direct port from GNU's coreutils, which are licensed under GPLv3, using BSD-licensed code (Go's own source).

### REQUIRES "github.com/ogier/pflag"
You can get `pflag` through `go get github.com/ogier/pflag`