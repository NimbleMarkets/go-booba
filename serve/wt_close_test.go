//go:build !js

package serve

import (
	"context"
	"crypto/tls"
	"encoding/binary"
	"encoding/json"
	"errors"
	"io"
	"net"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/quic-go/webtransport-go"

	"github.com/NimbleMarkets/go-booba/sip"
)

// slowReaderSession emits "hello" as its only output and then signals done.
type slowReaderSession struct {
	ctx  context.Context
	outR *io.PipeReader
	done chan struct{}
}

func (s *slowReaderSession) Context() context.Context { return s.ctx }
func (s *slowReaderSession) OutputReader() io.Reader  { return s.outR }
func (s *slowReaderSession) InputWriter() io.Writer   { return io.Discard }
func (s *slowReaderSession) Resize(_, _ int)          {}
func (s *slowReaderSession) WindowSize() WindowSize   { return WindowSize{Width: 80, Height: 24} }
func (s *slowReaderSession) Done() <-chan struct{}    { return s.done }
func (s *slowReaderSession) Close() error {
	_ = s.outR.CloseWithError(io.EOF)
	return nil
}

func freePorts(t *testing.T) (tcpPort, udpPort int) {
	t.Helper()
	tcpL, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen tcp: %v", err)
	}
	tcpPort = tcpL.Addr().(*net.TCPAddr).Port
	_ = tcpL.Close()

	udpL, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 0})
	if err != nil {
		t.Fatalf("listen udp: %v", err)
	}
	udpPort = udpL.LocalAddr().(*net.UDPAddr).Port
	_ = udpL.Close()
	return tcpPort, udpPort
}

func readWTFrame(stream *webtransport.Stream) (byte, []byte, error) {
	lenBuf := make([]byte, 4)
	if _, err := io.ReadFull(stream, lenBuf); err != nil {
		return 0, nil, err
	}
	body := make([]byte, binary.BigEndian.Uint32(lenBuf))
	if _, err := io.ReadFull(stream, body); err != nil {
		return 0, nil, err
	}
	return body[0], body[1:], nil
}

// TestHandleWTDeliversFramesToSlowReader pins the WebTransport teardown
// order. When the session workload finishes before the client has read its
// frames, the server must not close the WebTransport session until the
// client has had a chance to drain the stream: a session close makes the
// client's library cancel its stream reads and discard buffered frames.
func TestHandleWTDeliversFramesToSlowReader(t *testing.T) {
	if testing.Short() {
		t.Skip("WT requires UDP loopback; skipping in -short mode")
	}

	factory := func(ctx context.Context, _ WindowSize) (Session, error) {
		outR, outW := io.Pipe()
		done := make(chan struct{})
		go func() {
			defer close(done)
			defer func() { _ = outW.Close() }()
			_, _ = outW.Write([]byte("hello"))
		}()
		return &slowReaderSession{ctx: ctx, outR: outR, done: done}, nil
	}

	tcpPort, udpPort := freePorts(t)
	cfg := DefaultConfig()
	cfg.Host = "127.0.0.1"
	cfg.Port = tcpPort
	cfg.HTTP3Port = udpPort
	srv := NewServer(cfg, WithSessionFactory(factory))

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	go func() { _ = srv.Serve(ctx, nil) }()

	dialer := webtransport.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true, NextProtos: []string{"h3"}}, //nolint:gosec // test server uses a self-signed cert
	}
	wtURL := "https://127.0.0.1:" + strconv.Itoa(udpPort) + "/wt"
	var sess *webtransport.Session
	deadline := time.Now().Add(5 * time.Second)
	for {
		var err error
		_, sess, err = dialer.Dial(ctx, wtURL, nil)
		if err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("dial: %v", err)
		}
		time.Sleep(50 * time.Millisecond)
	}
	defer func() { _ = sess.CloseWithError(0, "") }()

	stream, err := sess.OpenStreamSync(ctx)
	if err != nil {
		t.Fatalf("open stream: %v", err)
	}
	resize, _ := json.Marshal(sip.ResizeMessage{Cols: 80, Rows: 24})
	if _, err := stream.Write(sip.EncodeWTMessage(sip.MsgResize, resize)); err != nil {
		t.Fatalf("write resize: %v", err)
	}

	// Let the server run the whole session to completion before we read.
	time.Sleep(500 * time.Millisecond)

	var types []byte
	sawHello, sawClose := false, false
	for {
		msgType, payload, err := readWTFrame(stream)
		if err != nil {
			if !errors.Is(err, io.EOF) {
				t.Errorf("read frame: %v (after types %v)", err, types)
			}
			break
		}
		types = append(types, msgType)
		switch msgType {
		case sip.MsgOutput:
			if strings.Contains(string(payload), "hello") {
				sawHello = true
			}
		case sip.MsgClose:
			sawClose = true
		}
	}
	if !sawHello {
		t.Errorf("no output frame containing 'hello'; frame types %v", types)
	}
	if !sawClose {
		t.Errorf("no close frame; frame types %v", types)
	}
}
