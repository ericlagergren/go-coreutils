package main

import (
	"bytes"
	"log"
	"os/exec"
	"testing"
)

func TestLookupUserName(t *testing.T) {

	got := lookupUserName()

	var out bytes.Buffer
	cmd := exec.Command("whoami")
	cmd.Stdout = &out
	err := cmd.Run()
	if err != nil {
		log.Fatal(err)
	}

	want := out.String()
	want = want[:len(want)-1] // remove \n

	if got != want {
		t.Errorf("whoami == %q, want %q", got, want)
	}

}
