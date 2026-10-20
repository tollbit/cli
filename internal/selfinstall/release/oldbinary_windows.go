//go:build windows

package release

import (
	"os"
	"path/filepath"
)

// RemoveOldBinary deletes the binary a previous install moved aside. It
// cannot be deleted while that run is still going, so the next run does it.
func RemoveOldBinary() {
	exe, err := os.Executable()
	if err != nil {
		return
	}
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		exe = resolved
	}
	_ = os.Remove(exe + oldSuffix)
}
