package main

import (
	"bytes"
	"os"
	"testing"
)

func TestCalc_md5sum(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"hello, world", "e4d7f1b4ed2e42d15898f4b27b019da4"},
		{"ad3344412123123fasdfasdf", "353a3336352ae74136ef5b37e4091c4c"},
		{"333dddf213sfasdfasdfasfd\n", "106f56f032f7d29af6af98eeb24d5d2c"},
	}

	for _, v := range cases {
		buf := bytes.NewBufferString(v.in)
		sum := calc_md5sum(buf)
		if sum != v.want {
			t.Errorf("md5sum (%#v) == %#v, want %#v", v.in, sum, v.want)
		} else {
			t.Logf("md5sum (%#v), expect %#v, got %#v\n", v.in, v.want, sum)
		}
	}
}

func TestCheck_md5sum(t *testing.T) {
	os.Chdir("testdata")

	cases := map[string]bool{
		"md5.sum":   true,
		"md5_f.sum": false,
	}

	for k, v := range cases {
		fp, err := os.Open(k)
		if err != nil {
			t.Errorf("%s", err.Error())
		}

		defer fp.Close()

		if r := check_md5sum_f(fp); r != v {
			t.Fail()
		} else {
			t.Logf("check result: expect %#v, got %#v\n", v, r)
		}
	}
}
