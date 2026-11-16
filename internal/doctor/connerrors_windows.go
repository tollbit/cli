//go:build windows

package doctor

import "golang.org/x/sys/windows"

// Windows reports socket failures as WSA codes, which do not match the
// syscall package's ECONNREFUSED and the like.
var (
	// refusedErrors are the OS errors for a host that refused or reset the
	// connection.
	refusedErrors = []error{windows.WSAECONNREFUSED, windows.WSAECONNRESET}
	// blockedErrors are the OS errors for a connection it would not make or
	// has no route for.
	blockedErrors = []error{windows.WSAEACCES, windows.WSAENETUNREACH, windows.WSAEHOSTUNREACH}
)
