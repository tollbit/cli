//go:build !windows

package release

// RemoveOldBinary deletes the binary a Windows install moved aside. Other
// platforms replace the binary in one rename and leave nothing behind.
func RemoveOldBinary() {}
