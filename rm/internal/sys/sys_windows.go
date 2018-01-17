// +build windows

package sys

import "os"

func IsRoot(_ os.FileInfo) bool    { return false }
func DiffFS(_, _ os.FileInfo) bool { return false }
