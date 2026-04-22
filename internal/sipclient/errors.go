package sipclient

import "errors"

// Classification sentinels wrap errors so the main entry point can map them
// to the exit codes specified in the design document:
//
//	0 — clean disconnect (no error)
//	1 — connect or TLS handshake failure (ErrConnect)
//	2 — protocol error, malformed frame, or unknown type (ErrProtocol)
//	3 — unexpected transport close (ErrTransport)
var (
	ErrConnect   = errors.New("connect failure")
	ErrProtocol  = errors.New("protocol error")
	ErrTransport = errors.New("transport error")
)
