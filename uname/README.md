The -o, --operating-system switch should work just fine for most
standard OSs.

If you need to set a custom value, please install the tool as such:

`go install --tags=ostypes -ldflags '-X main.hostOS=FOO'`

If you do not know the value you need to set, please run `go generate` prior
to installing this command.