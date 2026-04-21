package sipclient

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/NimbleMarkets/go-booba/sip"
)

// ErrSessionClosed is returned by Router.Route when a MsgClose frame is
// received. Callers treat it as a clean termination signal (exit 0), not an
// error.
var ErrSessionClosed = errors.New("session closed by server")

// FrameHandler receives decoded frames from the router. Implementations wire
// each method to the appropriate side-effect (tty write, title emit, flag
// push). Ping/Pong is handled by the router itself via the Pong callback, so
// handlers do not need to know about it.
type FrameHandler interface {
	HandleOutput(payload []byte)
	HandleTitle(title string)
	HandleOptions(opts sip.OptionsMessage)
	HandleKittyFlags(flags int)
	HandleClose(payload []byte)
}

// Router decodes and dispatches a single frame at a time. It is stateless
// beyond its fields, so callers may share one Router between goroutines if
// the Handler's methods are safe for concurrent use (the interactive
// handler is not; the dump-frames handler is).
type Router struct {
	Handler FrameHandler
	// Pong is called when a MsgPing is received. If it returns an error,
	// Route propagates it.
	Pong func() error
	// Debug, if non-nil, is called for every frame (including unknown
	// types). When Debug is set, unknown types are logged but do NOT return
	// an error.
	Debug func(msgType byte, payload []byte)
}

// Route dispatches a single frame. It returns ErrSessionClosed for MsgClose
// (callers should treat this as success), an error for malformed or unknown
// frames (unless Debug is set), and nil otherwise.
func (r *Router) Route(msgType byte, payload []byte) error {
	if r.Debug != nil {
		r.Debug(msgType, payload)
	}
	switch msgType {
	case sip.MsgOutput:
		r.Handler.HandleOutput(payload)
		return nil
	case sip.MsgTitle:
		r.Handler.HandleTitle(string(payload))
		return nil
	case sip.MsgOptions:
		var opts sip.OptionsMessage
		if err := json.Unmarshal(payload, &opts); err != nil {
			return fmt.Errorf("decode options: %w", err)
		}
		r.Handler.HandleOptions(opts)
		return nil
	case sip.MsgKittyKbd:
		var kk sip.KittyKbdMessage
		if err := json.Unmarshal(payload, &kk); err != nil {
			return fmt.Errorf("decode kitty: %w", err)
		}
		r.Handler.HandleKittyFlags(kk.Flags)
		return nil
	case sip.MsgPing:
		if r.Pong == nil {
			return errors.New("ping received but no Pong callback set")
		}
		return r.Pong()
	case sip.MsgClose:
		r.Handler.HandleClose(payload)
		return ErrSessionClosed
	default:
		if r.Debug != nil {
			return nil
		}
		return fmt.Errorf("unknown message type %q", msgType)
	}
}
