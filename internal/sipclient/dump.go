package sipclient

import (
	"encoding/base64"
	"encoding/json"
	"io"
	"sync"

	"github.com/NimbleMarkets/go-booba/sip"
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
