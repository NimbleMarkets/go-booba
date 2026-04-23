package sipclient

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/quic-go/quic-go"
	"github.com/quic-go/quic-go/http3"
	"github.com/quic-go/webtransport-go"

	"github.com/NimbleMarkets/go-booba/sip"
)

// generateTestCert creates a self-signed ECDSA P-256 certificate valid for
// 127.0.0.1 suitable for in-process test use.
func generateTestCert(t *testing.T) tls.Certificate {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("ecdsa.GenerateKey: %v", err)
	}
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		t.Fatalf("rand.Int: %v", err)
	}
	template := &x509.Certificate{
		SerialNumber: serial,
		Subject:      pkix.Name{CommonName: "127.0.0.1"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(10 * 24 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		IPAddresses:  []net.IP{net.ParseIP("127.0.0.1")},
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("x509.CreateCertificate: %v", err)
	}
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyBytes, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		t.Fatalf("x509.MarshalECPrivateKey: %v", err)
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyBytes})
	cert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		t.Fatalf("tls.X509KeyPair: %v", err)
	}
	return cert
}

// serverHandoff carries the server-side session and stream from the HTTP handler.
type serverHandoff struct {
	sess   *webtransport.Session
	stream *webtransport.Stream
}

// newWTPair returns a paired (server, client) wtFrameConn for testing.
// It starts a real in-process webtransport.Server on a loopback UDP socket.
//
// Stream establishment: the client opens a bidi stream and immediately writes a
// bootstrap MsgPing frame. Because webtransport-go sends the WT stream header
// lazily (on the first Write), the Write is required to make the stream visible
// to the server's AcceptStream. The bootstrap frame is drained server-side
// before newWTPair returns so test cases start with a clean buffer.
func newWTPair(t *testing.T) (server, client FrameConn, cleanup func()) {
	t.Helper()

	cert := generateTestCert(t)

	// Bind a UDP socket on a random port.
	laddr, err := net.ResolveUDPAddr("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("ResolveUDPAddr: %v", err)
	}
	udpConn, err := net.ListenUDP("udp", laddr)
	if err != nil {
		t.Fatalf("ListenUDP: %v", err)
	}
	addr := udpConn.LocalAddr().(*net.UDPAddr)

	// handoffCh carries the server session+stream once the handler sets them up.
	handoffCh := make(chan serverHandoff, 1)
	// readyCh is closed when the test is done, allowing the handler to exit.
	readyCh := make(chan struct{})

	mux := http.NewServeMux()
	var wtSrv *webtransport.Server

	tlsCfg := &tls.Config{
		Certificates: []tls.Certificate{cert},
		NextProtos:   []string{"h3"},
	}

	wtSrv = &webtransport.Server{
		H3: &http3.Server{
			TLSConfig:       tlsCfg,
			EnableDatagrams: true,
			Handler:         mux,
			QUICConfig: &quic.Config{
				EnableDatagrams:                  true,
				EnableStreamResetPartialDelivery: true,
			},
		},
		CheckOrigin: func(r *http.Request) bool { return true },
	}
	webtransport.ConfigureHTTP3Server(wtSrv.H3)

	// The handler upgrades, then waits for the client to open a stream.
	// The client writes a bootstrap ping frame to make the stream visible to
	// AcceptStream (stream headers are sent lazily on first Write).
	mux.HandleFunc("/wt", func(w http.ResponseWriter, r *http.Request) {
		sess, err := wtSrv.Upgrade(w, r)
		if err != nil {
			t.Errorf("Upgrade: %v", err)
			return
		}
		stream, err := sess.AcceptStream(context.Background())
		if err != nil {
			t.Errorf("AcceptStream: %v", err)
			return
		}
		handoffCh <- serverHandoff{sess: sess, stream: stream}
		// Keep the handler alive (and thus the session) until the test is done.
		select {
		case <-readyCh:
		case <-sess.Context().Done():
		}
	})

	go func() {
		_ = wtSrv.Serve(udpConn)
	}()

	// Client-side: dial and get a session.
	dialCtx, dialCancel := context.WithTimeout(context.Background(), 10*time.Second)

	wtURL := "https://127.0.0.1:" + itoa(addr.Port) + "/wt"
	dialer := &webtransport.Dialer{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true, //nolint:gosec // intentional for loopback test
			NextProtos:         []string{"h3"},
		},
		QUICConfig: &quic.Config{
			EnableDatagrams:                  true,
			EnableStreamResetPartialDelivery: true,
		},
	}
	_, clientSess, err := dialer.Dial(dialCtx, wtURL, nil)
	if err != nil {
		dialCancel()
		_ = wtSrv.Close()
		_ = udpConn.Close()
		close(readyCh)
		t.Fatalf("Dial: %v", err)
	}

	// Client opens a bidi stream. The stream header is sent lazily on first
	// Write, so the client writes a zero-length WT message to trigger the
	// header. This makes the stream visible to the server's AcceptStream.
	openCtx, openCancel := context.WithTimeout(context.Background(), 5*time.Second)
	clientStream, err := clientSess.OpenStreamSync(openCtx)
	openCancel()
	if err != nil {
		dialCancel()
		_ = wtSrv.Close()
		close(readyCh)
		t.Fatalf("OpenStreamSync: %v", err)
	}
	// Send a zero-length ping frame to flush the WT stream header to the wire,
	// making the stream visible to the server's conn.AcceptStream loop.
	if _, werr := clientStream.Write(sip.EncodeWTMessage(sip.MsgPing, nil)); werr != nil {
		dialCancel()
		_ = wtSrv.Close()
		close(readyCh)
		t.Fatalf("stream header write: %v", werr)
	}

	// Wait for the server handler to have accepted the stream.
	var ho serverHandoff
	select {
	case ho = <-handoffCh:
	case <-time.After(5 * time.Second):
		dialCancel()
		_ = wtSrv.Close()
		close(readyCh)
		t.Fatal("timed out waiting for server stream handoff")
	}

	serverFC := newWTFrameConn(ho.sess, ho.stream)
	clientFC := newWTFrameConn(clientSess, clientStream)

	// Drain the bootstrap ping frame we wrote to make the stream visible.
	// The server's ReadFrame will see it first; consume it here so tests
	// start with a clean state.
	bootstrapCtx, bootstrapCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer bootstrapCancel()
	if _, _, err := serverFC.ReadFrame(bootstrapCtx); err != nil {
		dialCancel()
		_ = wtSrv.Close()
		close(readyCh)
		t.Fatalf("drain bootstrap frame: %v", err)
	}

	cleanup = func() {
		_ = clientFC.CloseNow()
		_ = serverFC.CloseNow()
		dialCancel()
		close(readyCh)
		_ = wtSrv.Close()
	}
	return serverFC, clientFC, cleanup
}

// itoa converts an int to string without importing strconv.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	buf := [20]byte{}
	pos := len(buf)
	for n > 0 {
		pos--
		buf[pos] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[pos:])
}

func TestWTFrameConn_RoundTrip(t *testing.T) {
	if testing.Short() {
		t.Skip("WT test requires UDP loopback; skipping in -short mode")
	}

	server, client, cleanup := newWTPair(t)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cases := []struct {
		name    string
		msgType byte
		payload []byte
	}{
		{"output", sip.MsgOutput, []byte("hello world")},
		{"input", sip.MsgInput, []byte("")},
		{"large", sip.MsgOutput, bytes.Repeat([]byte("x"), 64*1024)},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if err := client.WriteFrame(ctx, c.msgType, c.payload); err != nil {
				t.Fatalf("WriteFrame: %v", err)
			}
			typ, payload, err := server.ReadFrame(ctx)
			if err != nil {
				t.Fatalf("ReadFrame: %v", err)
			}
			if typ != c.msgType {
				t.Errorf("type = %q; want %q", typ, c.msgType)
			}
			if !bytes.Equal(payload, c.payload) && (len(payload) != 0 || len(c.payload) != 0) {
				t.Errorf("payload mismatch: got %d bytes, want %d", len(payload), len(c.payload))
			}
		})
	}
}

func TestWTFrameConn_NormalCloseDetected(t *testing.T) {
	if testing.Short() {
		t.Skip("WT test requires UDP loopback; skipping in -short mode")
	}

	server, client, cleanup := newWTPair(t)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Close(StatusNormal, "bye"); err != nil {
		t.Fatalf("server.Close: %v", err)
	}

	_, _, err := client.ReadFrame(ctx)
	if err == nil {
		t.Fatal("expected error after peer close")
	}
	if !IsNormalClose(err) {
		t.Errorf("IsNormalClose(%v) = false; want true", err)
	}
}
