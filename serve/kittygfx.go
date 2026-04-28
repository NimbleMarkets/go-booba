package serve

import (
	"bytes"
	"encoding/base64"
	"image"
	"image/png"
	"strconv"
	"strings"
)

// kittyGfxTranscoder is a stateful byte-stream filter that intercepts kitty
// graphics protocol APC sequences carrying PNG payloads (f=100) and rewrites
// them as raw RGBA payloads (f=32). The wasm build of ghostty-vt used by
// ghostty-web does not link wuffs, so PNGs are rejected with
// "EINVAL: unsupported format" — this transcoder makes PNG-emitting TUI
// libraries (kitten icat, ntcharts, etc.) work transparently.
//
// Single- and multi-chunk APC transmissions are both supported. APC sequences
// using formats other than f=100 pass through untouched. So do non-graphics
// bytes.
//
// One transcoder per session. Not safe for concurrent use.
type kittyGfxTranscoder struct {
	state   transcoderState
	apcBody bytes.Buffer
	loading *pngLoading
}

type transcoderState int

const (
	stPass transcoderState = iota
	stEsc
	stEscUnderscore
	stApc
	stApcEsc
)

type pngLoading struct {
	// metadata from the first chunk's control header. We rewrite f= and the
	// dimensions on emit; everything else (a=, q=, image_id, placement, etc.)
	// passes through.
	meta map[string]string
	// base64-encoded payload accumulated across chunks. Per the kitty graphics
	// protocol, callers split the base64 string at arbitrary byte boundaries —
	// so chunks aren't individually decodable; we concat and decode once.
	encoded bytes.Buffer
	// drop is true when the entire multi-chunk transmission should be silently
	// suppressed (e.g. U=1 virtual-placement transmissions that ghostty-web
	// can't render).
	drop bool
}

// kittyEmitChunkSize bounds the base64 payload size in each emitted APC
// transmission chunk. The protocol allows up to ~4 KiB per chunk in practice;
// staying under that keeps us well within most parsers' buffers.
const kittyEmitChunkSize = 4096

// maxAPCBody caps the bytes we'll buffer inside a single APC body. If a peer
// emits an APC start without ever sending a terminator we don't want to grow
// indefinitely. 16 MiB easily covers any realistic image transmission while
// bounding worst-case memory.
const maxAPCBody = 16 * 1024 * 1024

// Filter consumes a slice of PTY bytes and returns the (possibly rewritten)
// stream. The output may be larger or smaller than the input; callers should
// chunk the result against any downstream message-size limit.
func (t *kittyGfxTranscoder) Filter(in []byte) []byte {
	var out bytes.Buffer
	for _, b := range in {
		switch t.state {
		case stPass:
			if b == 0x1b {
				t.state = stEsc
			} else {
				out.WriteByte(b)
			}
		case stEsc:
			if b == '_' {
				t.state = stEscUnderscore
			} else {
				out.WriteByte(0x1b)
				out.WriteByte(b)
				t.state = stPass
			}
		case stEscUnderscore:
			if b == 'G' {
				t.state = stApc
				t.apcBody.Reset()
			} else {
				// Some other APC type (e.g. _Z for tmux); we don't intercept,
				// pass it through and stay in passthrough.
				out.WriteByte(0x1b)
				out.WriteByte('_')
				out.WriteByte(b)
				t.state = stPass
			}
		case stApc:
			switch b {
			case 0x1b:
				t.state = stApcEsc
			case 0x07:
				// BEL terminator
				t.flushAPC(&out)
				t.state = stPass
			default:
				if t.apcBody.Len() >= maxAPCBody {
					// Runaway APC. Abort: emit whatever we have as-is and
					// resume passthrough so we don't OOM on a missing terminator.
					t.abortAPC(&out)
					out.WriteByte(b)
					t.state = stPass
				} else {
					t.apcBody.WriteByte(b)
				}
			}
		case stApcEsc:
			if b == '\\' {
				// ST terminator (ESC \)
				t.flushAPC(&out)
				t.state = stPass
			} else {
				if t.apcBody.Len()+2 >= maxAPCBody {
					t.abortAPC(&out)
					out.WriteByte(0x1b)
					out.WriteByte(b)
					t.state = stPass
				} else {
					t.apcBody.WriteByte(0x1b)
					t.apcBody.WriteByte(b)
					t.state = stApc
				}
			}
		}
	}
	return out.Bytes()
}

// abortAPC drops accumulated APC state without emitting an APC, used when a
// pathological APC exceeds maxAPCBody without a terminator. We emit the raw
// introducer + body so the receiver sees the bytes (and likely errors loudly)
// rather than silently swallowing them.
func (t *kittyGfxTranscoder) abortAPC(out *bytes.Buffer) {
	out.WriteString("\x1b_G")
	out.Write(t.apcBody.Bytes())
	t.apcBody.Reset()
	t.loading = nil
}

// flushAPC processes one complete APC body (between \x1b_G and the terminator),
// either rewriting it (PNG path) or emitting it unchanged.
func (t *kittyGfxTranscoder) flushAPC(out *bytes.Buffer) {
	body := t.apcBody.Bytes()
	semi := bytes.IndexByte(body, ';')
	var metaRaw, payload []byte
	if semi == -1 {
		metaRaw = body
	} else {
		metaRaw = body[:semi]
		payload = body[semi+1:]
	}
	meta := parseKittyMeta(metaRaw)

	// If we're already mid-load, every subsequent APC contributes payload
	// regardless of its declared format — only the first chunk has metadata.
	if t.loading != nil {
		if t.loading.drop {
			// Continuation of a transmission we're suppressing; skip.
		} else {
			t.appendChunk(payload)
		}
		if isFinalChunk(meta) {
			if !t.loading.drop {
				t.emitDecoded(out)
			}
			t.loading = nil
		}
		return
	}

	// Drop kitty graphics transmissions with U=1 (Unicode placeholder mode).
	// ghostty-web's renderer doesn't draw virtual placements, so transmitting
	// (potentially many MB of) image data the browser can't display only
	// pumps wasm memory and causes OOB during rapid redraws on resize.
	// Without this drop, ntcharts-style demos accumulate image state and
	// eventually crash the renderer.
	if meta["U"] == "1" {
		t.loading = &pngLoading{meta: meta, drop: true}
		if isFinalChunk(meta) {
			t.loading = nil
		}
		return
	}

	if meta["f"] != "100" {
		// Not a PNG transmission. Pass through verbatim.
		writeAPC(out, body)
		return
	}

	// First (or only) chunk of a PNG transmission. Start accumulating.
	t.loading = &pngLoading{meta: meta}
	t.appendChunk(payload)
	if isFinalChunk(meta) {
		t.emitDecoded(out)
		t.loading = nil
	}
}

func (t *kittyGfxTranscoder) appendChunk(payload []byte) {
	t.loading.encoded.Write(payload)
}

func (t *kittyGfxTranscoder) emitDecoded(out *bytes.Buffer) {
	raw, err := decodeBase64Tolerant(t.loading.encoded.Bytes())
	if err != nil {
		t.passthroughLoading(out)
		return
	}
	img, err := png.Decode(bytes.NewReader(raw))
	if err != nil {
		// Decode failed. Pass through what we received as a best-effort PNG
		// transmission so the wasm side reports its own error rather than
		// silently dropping.
		t.passthroughLoading(out)
		return
	}
	bounds := img.Bounds()
	width, height := bounds.Dx(), bounds.Dy()

	// Convert to RGBA in a single pass.
	rgba, ok := img.(*image.NRGBA)
	var pix []byte
	if ok && bounds.Min.X == 0 && bounds.Min.Y == 0 && rgba.Stride == width*4 {
		pix = rgba.Pix
	} else {
		dst := image.NewNRGBA(image.Rect(0, 0, width, height))
		for y := 0; y < height; y++ {
			for x := 0; x < width; x++ {
				dst.Set(x, y, img.At(x+bounds.Min.X, y+bounds.Min.Y))
			}
		}
		pix = dst.Pix
	}

	// Build new metadata: format becomes RGBA raw (32), width/height come from
	// the decoded image. Drop chunk markers and the original size hints; we'll
	// add them back when writing chunks.
	meta := t.loading.meta
	meta["f"] = "32"
	meta["s"] = strconv.Itoa(width)
	meta["v"] = strconv.Itoa(height)
	delete(meta, "m")
	delete(meta, "o") // no compression on the rewritten payload

	encoded := base64.StdEncoding.EncodeToString(pix)
	writeChunkedAPC(out, meta, encoded)
}

// passthroughLoading writes the originally-received bytes as a single APC
// without transcoding. Used when PNG decode fails so the receiver still sees
// the (broken) frame and can log/error normally.
func (t *kittyGfxTranscoder) passthroughLoading(out *bytes.Buffer) {
	writeChunkedAPC(out, t.loading.meta, t.loading.encoded.String())
}

// decodeBase64Tolerant decodes a base64 string, accepting either standard
// padded or unpadded encoding. The kitty graphics protocol officially uses
// padded base64 but unpadded variants show up in practice.
func decodeBase64Tolerant(in []byte) ([]byte, error) {
	if decoded, err := base64.StdEncoding.DecodeString(string(in)); err == nil {
		return decoded, nil
	}
	return base64.RawStdEncoding.DecodeString(string(in))
}

// writeAPC emits one untouched APC sequence: \x1b_G <body> \x1b\
func writeAPC(out *bytes.Buffer, body []byte) {
	out.WriteString("\x1b_G")
	out.Write(body)
	out.WriteString("\x1b\\")
}

// writeChunkedAPC emits a kitty graphics transmission. If the encoded payload
// fits in one chunk it goes out as-is; otherwise it's split into m=1
// continuation chunks with a final m=0 terminator (per the kitty graphics
// protocol).
func writeChunkedAPC(out *bytes.Buffer, meta map[string]string, encoded string) {
	if len(encoded) <= kittyEmitChunkSize {
		out.WriteString("\x1b_G")
		writeMeta(out, meta)
		out.WriteByte(';')
		out.WriteString(encoded)
		out.WriteString("\x1b\\")
		return
	}

	// First chunk: full metadata + m=1
	first := encoded[:kittyEmitChunkSize]
	out.WriteString("\x1b_G")
	writeMetaWithOverride(out, meta, "m", "1")
	out.WriteByte(';')
	out.WriteString(first)
	out.WriteString("\x1b\\")

	// Middle and final chunks carry no metadata except m=.
	i := kittyEmitChunkSize
	for i+kittyEmitChunkSize < len(encoded) {
		out.WriteString("\x1b_Gm=1;")
		out.WriteString(encoded[i : i+kittyEmitChunkSize])
		out.WriteString("\x1b\\")
		i += kittyEmitChunkSize
	}
	out.WriteString("\x1b_Gm=0;")
	out.WriteString(encoded[i:])
	out.WriteString("\x1b\\")
}

// parseKittyMeta parses "k=v,k=v,..." into a map. Empty input returns an empty
// (non-nil) map so subsequent lookups don't panic on nil-map writes.
func parseKittyMeta(in []byte) map[string]string {
	m := make(map[string]string, 8)
	if len(in) == 0 {
		return m
	}
	for _, pair := range strings.Split(string(in), ",") {
		eq := strings.IndexByte(pair, '=')
		if eq < 0 {
			continue
		}
		m[pair[:eq]] = pair[eq+1:]
	}
	return m
}

// writeMeta serialises a metadata map to "k=v,k=v,..." into out.
func writeMeta(out *bytes.Buffer, m map[string]string) {
	first := true
	for k, v := range m {
		if !first {
			out.WriteByte(',')
		}
		first = false
		out.WriteString(k)
		out.WriteByte('=')
		out.WriteString(v)
	}
}

// writeMetaWithOverride serialises m but forces the value of the named key.
// The key is included even if not in the map.
func writeMetaWithOverride(out *bytes.Buffer, m map[string]string, key, value string) {
	wroteKey := false
	first := true
	for k, v := range m {
		if !first {
			out.WriteByte(',')
		}
		first = false
		out.WriteString(k)
		out.WriteByte('=')
		if k == key {
			out.WriteString(value)
			wroteKey = true
		} else {
			out.WriteString(v)
		}
	}
	if !wroteKey {
		if !first {
			out.WriteByte(',')
		}
		out.WriteString(key)
		out.WriteByte('=')
		out.WriteString(value)
	}
}

// isFinalChunk returns true when this APC is either standalone or the last
// chunk of a chunked transmission. Per the kitty graphics protocol, m=1 means
// "more chunks follow" and m=0 (or m absent) means "this is the last".
func isFinalChunk(meta map[string]string) bool {
	m, ok := meta["m"]
	if !ok {
		return true
	}
	return m == "0"
}
