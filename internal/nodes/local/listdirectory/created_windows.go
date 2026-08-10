//go:build windows

package listdirectory

import (
	"io/fs"
	"syscall"
	"time"
)

func creationTime(info fs.FileInfo) (time.Time, bool) {
	attributes, ok := info.Sys().(*syscall.Win32FileAttributeData)
	if !ok {
		return time.Time{}, false
	}
	return time.Unix(0, attributes.CreationTime.Nanoseconds()), true
}
