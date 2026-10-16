package network

import "syscall"

// Winsock reports these under its own numbers: WSAECONNREFUSED,
// WSAENETUNREACH and WSAEHOSTUNREACH.
var (
	errRefused     = []error{syscall.ECONNREFUSED, syscall.Errno(10061)}
	errUnreachable = []error{syscall.ENETUNREACH, syscall.EHOSTUNREACH, syscall.Errno(10051), syscall.Errno(10065)}
)
