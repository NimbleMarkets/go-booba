package sipclient

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"sync"

	"github.com/NimbleMarkets/go-booba/sip"
	"github.com/coder/websocket"
)

// DumpHandler implements FrameHandler by writing one JSON line per frame to
// its output. It serializes concurrent writes so frames are not interleaved.
type DumpHandler struct {
	mu  sync.Mutex
	w   io.Writer
	enc *json.Encoder
}

// NewDumpHandler returns a handler that writes compact JSON lines to w. The
// encoder's newline terminator keeps the output `jq --stream`-friendly.
func NewDumpHandler(w io.Writer) *DumpHandler {
	enc := json.NewEncoder(w)
	// json.Encoder writes a newline after every value by default, so each
	// Encode call produces exactly one line — ideal for --dump-frames.
	return &DumpHandler{w: w, enc: enc}
}

func (h *DumpHandler) emit(v any) {
	h.mu.Lock()
	defer h.mu.Unlock()
	_ = h.enc.Encode(v)
}

func (h *DumpHandler) HandleOutput(payload []byte) {
	h.emit(struct {
		Type string `json:"type"`
		Data string `json:"data"`
	}{"output", base64.StdEncoding.EncodeToString(payload)})
}

func (h *DumpHandler) HandleTitle(title string) {
	h.emit(struct {
		Type  string `json:"type"`
		Title string `json:"title"`
	}{"title", title})
}

func (h *DumpHandler) HandleOptions(opts sip.OptionsMessage) {
	h.emit(struct {
		Type     string `json:"type"`
		ReadOnly bool   `json:"readOnly"`
	}{"options", opts.ReadOnly})
}

func (h *DumpHandler) HandleKittyFlags(flags int) {
	h.emit(struct {
		Type  string `json:"type"`
		Flags int    `json:"flags"`
	}{"kitty", flags})
}

func (h *DumpHandler) HandleClose(payload []byte) {
	h.emit(struct {
		Type string `json:"type"`
		Data string `json:"data"`
	}{"close", base64.StdEncoding.EncodeToString(payload)})
}

// Static assertion that *DumpHandler satisfies FrameHandler.
var _ FrameHandler = (*DumpHandler)(nil)

// RunDump opens a connection to opts.URL and writes decoded frames as JSON
// lines to stdout until the server closes, --dump-timeout elapses, or ctx is
// canceled. Returns nil on clean close, non-nil on any other termination.
func RunDump(ctx context.Context, stdout, stderr io.Writer, opts *Options) error {
	target, err := ParseTargetURL(opts.URL)
	if err != nil {
		return err
	}
	headers, err := ParseHeaders(opts.Headers)
	if err != nil {
		return err
	}
	tlsCfg, err := BuildTLSConfig(opts.InsecureSkipVerify, opts.CAFile)
	if err != nil {
		return err
	}

	conn, err := Dial(ctx, DialOptions{
		Target:  target,
		Origin:  opts.Origin,
		Headers: headers,
		TLS:     tlsCfg,
		Timeout: opts.ConnectTimeout,
	})
	if err != nil {
		return err
	}
	defer func() { _ = conn.CloseNow() }()

	// Send an initial Resize so servers that gate output on resize (which
	// is most SIP-compatible servers) can start emitting frames. Use a
	// reasonable default 80x24 — dump mode is not attached to a real
	// terminal.
	{
		body, err := json.Marshal(sip.ResizeMessage{Cols: 80, Rows: 24})
		if err != nil {
			return fmt.Errorf("marshal resize: %w", err)
		}
		if err := conn.Write(ctx, websocket.MessageBinary, sip.EncodeWSMessage(sip.MsgResize, body)); err != nil {
			return fmt.Errorf("send initial resize: %w", err)
		}
	}

	// Optional: send a single MsgInput from --dump-input after connect.
	if opts.DumpInputPath != "" {
		data, err := os.ReadFile(opts.DumpInputPath)
		if err != nil {
			return fmt.Errorf("read --dump-input: %w", err)
		}
		if err := conn.Write(ctx, websocket.MessageBinary, sip.EncodeWSMessage(sip.MsgInput, data)); err != nil {
			return fmt.Errorf("send dump-input: %w", err)
		}
	}

	pumpCtx := ctx
	if opts.DumpTimeout > 0 {
		var cancel context.CancelFunc
		pumpCtx, cancel = context.WithTimeout(pumpCtx, opts.DumpTimeout)
		defer cancel()
	}

	handler := NewDumpHandler(stdout)
	router := &Router{
		Handler: handler,
		Pong: func() error {
			// Pong is best-effort in dump mode: if the write fails, the next
			// Read on this connection will surface the real error. Do NOT
			// terminate the dump session just because a Pong could not be
			// written.
			_ = conn.Write(pumpCtx, websocket.MessageBinary, sip.EncodeWSMessage(sip.MsgPong, nil))
			return nil
		},
	}
	if opts.Debug {
		router.Debug = func(t byte, p []byte) {
			_, _ = fmt.Fprintf(stderr, "debug: frame type=%q len=%d\n", t, len(p))
		}
	}

	for {
		_, data, err := conn.Read(pumpCtx)
		if err != nil {
			if errors.Is(err, context.DeadlineExceeded) {
				return nil // any deadline (ours or the caller's) is a clean end for dump mode
			}
			if errors.Is(err, context.Canceled) {
				return nil
			}
			// Websocket normal close is a clean termination in dump mode.
			if websocket.CloseStatus(err) == websocket.StatusNormalClosure {
				return nil
			}
			return fmt.Errorf("read frame: %w", err)
		}
		msgType, payload, derr := sip.DecodeWSMessage(data)
		if derr != nil {
			return fmt.Errorf("decode: %w", derr)
		}
		if err := router.Route(msgType, payload); err != nil {
			if errors.Is(err, ErrSessionClosed) {
				return nil
			}
			return err
		}
	}
}
