//go:build !windows

package network

import "syscall"

var (
	errRefused     = []error{syscall.ECONNREFUSED}
	errUnreachable = []error{syscall.ENETUNREACH, syscall.EHOSTUNREACH}
)
