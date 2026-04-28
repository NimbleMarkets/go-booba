package serve

import (
	"bytes"
	"encoding/base64"
	"image"
	"image/color"
	"image/png"
	"strings"
	"testing"
)

// makePNG returns a w x h PNG with a solid red opaque fill.
func makePNG(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.NRGBA{R: 255, G: 0, B: 0, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode test png: %v", err)
	}
	return buf.Bytes()
}

func TestTranscoder_PassthroughNoAPC(t *testing.T) {
	tr := &kittyGfxTranscoder{}
	in := []byte("hello world\r\n\x1b[31mred\x1b[0m\n")
	out := tr.Filter(in)
	if !bytes.Equal(out, in) {
		t.Fatalf("plain bytes were modified.\n in=%q\nout=%q", in, out)
	}
}

func TestTranscoder_PassthroughNonPngAPC(t *testing.T) {
	tr := &kittyGfxTranscoder{}
	in := []byte("\x1b_Ga=T,f=32,s=2,v=2;AAAAAAAA\x1b\\")
	out := tr.Filter(in)
	if !bytes.Equal(out, in) {
		t.Fatalf("non-PNG APC was modified.\n in=%q\nout=%q", in, out)
	}
}

func TestTranscoder_SingleChunkPng(t *testing.T) {
	pngBytes := makePNG(t, 4, 3)
	encoded := base64.StdEncoding.EncodeToString(pngBytes)
	in := []byte("\x1b_Ga=T,f=100,t=d;" + encoded + "\x1b\\")

	tr := &kittyGfxTranscoder{}
	out := tr.Filter(in)

	if bytes.Contains(out, []byte("f=100")) {
		t.Fatalf("output still contains f=100: %q", out)
	}
	if !bytes.Contains(out, []byte("f=32")) {
		t.Fatalf("output missing f=32: %q", out)
	}
	if !bytes.Contains(out, []byte("s=4")) || !bytes.Contains(out, []byte("v=3")) {
		t.Fatalf("output missing decoded dimensions s=4,v=3: %q", out)
	}

	// Pull the base64 payload out and verify it round-trips to a 4x3 RGBA buffer.
	first := bytes.Index(out, []byte("\x1b_G"))
	if first == -1 {
		t.Fatalf("no APC start in output")
	}
	semi := bytes.IndexByte(out[first:], ';')
	if semi == -1 {
		t.Fatalf("no payload separator")
	}
	end := bytes.Index(out[first+semi:], []byte("\x1b\\"))
	if end == -1 {
		t.Fatalf("no APC terminator")
	}
	payload := out[first+semi+1 : first+semi+end]
	decoded, err := base64.StdEncoding.DecodeString(string(payload))
	if err != nil {
		t.Fatalf("output payload not base64: %v", err)
	}
	if got, want := len(decoded), 4*3*4; got != want {
		t.Fatalf("payload size %d want %d (4x3 NRGBA)", got, want)
	}
	if decoded[0] != 0xff || decoded[1] != 0x00 || decoded[2] != 0x00 || decoded[3] != 0xff {
		t.Fatalf("first pixel not red opaque: %v", decoded[:4])
	}
}

func TestTranscoder_MultiChunkPng(t *testing.T) {
	pngBytes := makePNG(t, 16, 16)
	encoded := base64.StdEncoding.EncodeToString(pngBytes)

	// Split the base64 payload across three APC chunks.
	third := len(encoded) / 3
	c1, c2, c3 := encoded[:third], encoded[third:2*third], encoded[2*third:]
	in := strings.Builder{}
	in.WriteString("\x1b_Ga=T,f=100,t=d,m=1;")
	in.WriteString(c1)
	in.WriteString("\x1b\\")
	in.WriteString("\x1b_Gm=1;")
	in.WriteString(c2)
	in.WriteString("\x1b\\")
	in.WriteString("\x1b_Gm=0;")
	in.WriteString(c3)
	in.WriteString("\x1b\\")

	tr := &kittyGfxTranscoder{}
	out := tr.Filter([]byte(in.String()))

	if bytes.Contains(out, []byte("f=100")) {
		t.Fatalf("output still contains f=100: %q", out[:min(len(out), 200)])
	}
	if !bytes.Contains(out, []byte("f=32")) {
		t.Fatalf("output missing f=32")
	}
	if !bytes.Contains(out, []byte("s=16")) || !bytes.Contains(out, []byte("v=16")) {
		t.Fatalf("output missing s=16,v=16")
	}
}

func TestTranscoder_SplitAcrossReads(t *testing.T) {
	pngBytes := makePNG(t, 4, 3)
	encoded := base64.StdEncoding.EncodeToString(pngBytes)
	full := []byte("prefix\x1b_Ga=T,f=100,t=d;" + encoded + "\x1b\\suffix")

	// Concatenate Filter outputs for several arbitrary split points.
	tr := &kittyGfxTranscoder{}
	var out bytes.Buffer
	splits := []int{1, 3, 7, 9, 14, 25, 60}
	last := 0
	for _, s := range splits {
		if s > len(full) {
			break
		}
		out.Write(tr.Filter(full[last:s]))
		last = s
	}
	out.Write(tr.Filter(full[last:]))

	got := out.Bytes()
	if !bytes.HasPrefix(got, []byte("prefix")) {
		t.Fatalf("missing prefix")
	}
	if !bytes.HasSuffix(got, []byte("suffix")) {
		t.Fatalf("missing suffix; tail=%q", got[max(0, len(got)-32):])
	}
	if !bytes.Contains(got, []byte("f=32")) {
		t.Fatalf("split-stream did not transcode; got=%q", got)
	}
}

func TestTranscoder_DropsUnicodePlaceholderTransmissions(t *testing.T) {
	// Single-chunk U=1 transmission: should produce no output.
	pngBytes := makePNG(t, 8, 8)
	encoded := base64.StdEncoding.EncodeToString(pngBytes)
	in := []byte("before\x1b_Ga=T,U=1,f=100,t=d,i=42;" + encoded + "\x1b\\after")

	tr := &kittyGfxTranscoder{}
	out := tr.Filter(in)

	if bytes.Contains(out, []byte("\x1b_G")) {
		t.Fatalf("U=1 transmission was not dropped: %q", out)
	}
	if !bytes.Contains(out, []byte("before")) || !bytes.Contains(out, []byte("after")) {
		t.Fatalf("surrounding bytes lost: %q", out)
	}
}

func TestTranscoder_DropsUnicodePlaceholderMultichunk(t *testing.T) {
	pngBytes := makePNG(t, 16, 16)
	encoded := base64.StdEncoding.EncodeToString(pngBytes)
	third := len(encoded) / 3
	c1, c2, c3 := encoded[:third], encoded[third:2*third], encoded[2*third:]
	in := strings.Builder{}
	in.WriteString("\x1b_Ga=T,U=1,f=100,t=d,i=99,m=1;")
	in.WriteString(c1)
	in.WriteString("\x1b\\")
	in.WriteString("\x1b_Gm=1;")
	in.WriteString(c2)
	in.WriteString("\x1b\\")
	in.WriteString("\x1b_Gm=0;")
	in.WriteString(c3)
	in.WriteString("\x1b\\TAIL")

	tr := &kittyGfxTranscoder{}
	out := tr.Filter([]byte(in.String()))

	if bytes.Contains(out, []byte("\x1b_G")) {
		t.Fatalf("multi-chunk U=1 transmission leaked APC framing: %q", out)
	}
	if !bytes.HasSuffix(out, []byte("TAIL")) {
		t.Fatalf("trailing bytes lost: %q", out)
	}
}

func TestTranscoder_BadPngFallsThrough(t *testing.T) {
	garbage := base64.StdEncoding.EncodeToString([]byte("\x89PNG nope nope nope"))
	in := []byte("\x1b_Ga=T,f=100,t=d;" + garbage + "\x1b\\")

	tr := &kittyGfxTranscoder{}
	out := tr.Filter(in)

	// On decode failure we re-emit a passthrough APC of the originally received
	// bytes. The receiver gets the wasm error, not silence.
	if !bytes.Contains(out, []byte("\x1b_G")) || !bytes.Contains(out, []byte("\x1b\\")) {
		t.Fatalf("expected APC framing in fallback output: %q", out)
	}
}
