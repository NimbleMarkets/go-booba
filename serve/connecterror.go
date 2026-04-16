//go:build !js

package serve

import (
	"fmt"
	"net/http"
)

// ConnectError is returned by ConnectHandler implementations to control
// the rejection response. A plain error returned from a ConnectHandler
// is treated as &ConnectError{Status: http.StatusInternalServerError}.
type ConnectError struct {
	// Status is the HTTP status code written on the WS path. On the WT
	// path it is mapped to a QUIC error code (see WTErrorCode).
	Status int

	// Headers are written on the WS path before the body. Ignored on WT.
	Headers http.Header

	// Body is written on the WS path after the headers. Ignored on WT.
	Body string

	// WTCode, if non-zero, overrides the default WS-status to QUIC-code
	// mapping used by WTErrorCode.
	WTCode uint32

	// Cause is an optional underlying error included in the error string
	// and returned by Unwrap.
	Cause error
}

// Error implements the error interface.
func (e *ConnectError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("connect rejected: status=%d: %v", e.Status, e.Cause)
	}
	return fmt.Sprintf("connect rejected: status=%d", e.Status)
}

// Unwrap returns the underlying cause, if any.
func (e *ConnectError) Unwrap() error { return e.Cause }

// WTErrorCode returns the QUIC error code that should be used when
// closing a WebTransport session in response to this ConnectError.
// If WTCode is non-zero it is returned verbatim. Otherwise the default
// mapping is: 4xx → 0x01, 5xx → 0x02, anything else → 0x00.
func (e *ConnectError) WTErrorCode() uint32 {
	if e.WTCode != 0 {
		return e.WTCode
	}
	switch {
	case e.Status >= 400 && e.Status < 500:
		return 0x01
	case e.Status >= 500 && e.Status < 600:
		return 0x02
	default:
		return 0x00
	}
}
