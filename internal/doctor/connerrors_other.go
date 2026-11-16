//go:build !windows

package doctor

import "syscall"

var (
	// refusedErrors are the OS errors for a host that refused or reset the
	// connection.
	refusedErrors = []error{syscall.ECONNREFUSED, syscall.ECONNRESET}
	// blockedErrors are the OS errors for a connection it would not make or
	// has no route for.
	blockedErrors = []error{syscall.EPERM, syscall.EACCES, syscall.ENETUNREACH, syscall.EHOSTUNREACH}
)
