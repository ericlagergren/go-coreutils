// +build !linux !freebsd

package sys

func Fadvise(_ int) error { return nil }
