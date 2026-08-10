//go:build !windows

package listdirectory

import (
	"io/fs"
	"time"
)

func creationTime(_ fs.FileInfo) (time.Time, bool) {
	return time.Time{}, false
}
