package coreutils

import (
	"context"
	"errors"
	"io"
	"sync"
)

var cmdsMu sync.Mutex
var cmds = make(map[string]Runnable)

func Register(name string, r Runnable) {
	cmdsMu.Lock()
	defer cmdsMu.Unlock()
	if _, ok := cmds[name]; ok {
		panic("Register called with identical name: " + name)
	}
	cmds[name] = r
}

type Runnable func(ctx Context, args ...string) error

type Context struct {
	context.Context
	Dir    string
	GetEnv func(string) string
	Stdin  io.Reader
	Stdout io.Writer
	Stderr io.Writer
}

func Run(ctx Context, name string, args ...string) error {
	cmdsMu.Lock()
	fn := cmds[name]
	cmdsMu.Unlock()
	if fn == nil {
		return errors.New("invalid function ...")
	}
	return fn(ctx, args...)
}
