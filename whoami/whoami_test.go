package main

import (
	"os"
	"os/exec"
	"os/user"
	"strconv"
	"testing"
)

func TestWhoami(t *testing.T) {

	cmd := exec.Command("whoami")
	b, err := cmd.Output()
	if err != nil {
		t.Fatalf("%s\n", err.Error())
	}

	uid := strconv.Itoa(os.Geteuid())
	u, err := user.LookupId(uid)
	if err != nil {
		t.Fatalf("%s\n", err.Error())
	}

	if string(b[:len(b)-1]) != u.Username {
		t.Errorf("got: %q, want: %q", b, u.Username)
	}
}
