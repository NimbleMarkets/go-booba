//go:build !js

package serve

import (
	"bytes"
	"context"
	"crypto/tls"
	"io"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/quic-go/webtransport-go"
)

// Exercise the actual server initialization: webtransport-go rewrites the
// HTTP/3 settings from Config when Serve starts. Checking Config alone would
// miss a dependency change that stopped advertising Safari's required credit.
func TestWebTransportAdvertisesCreditAndReplenishesDataWindow(t *testing.T) {
	if testing.Short() {
		t.Skip("requires UDP loopback")
	}
	cert, err := GenerateSelfSignedCert("127.0.0.1")
	if err != nil {
		t.Fatal(err)
	}
	srv := NewServer(DefaultConfig())
	srv.certInfo = cert
	wt := srv.newWebTransportServer()
	wt.H3.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		session, err := wt.Upgrade(w, r)
		if err != nil {
			t.Errorf("upgrade: %v", err)
			return
		}
		stream, err := session.AcceptStream(r.Context())
		if err != nil {
			return
		}
		// Echo concurrently with client writes so neither direction depends on
		// buffering the entire payload within its initial flow-control window.
		_, _ = io.Copy(stream, stream)
		<-session.Context().Done()
	})
	udp, err := net.ListenPacket("udp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})
	go func() { defer close(done); _ = wt.Serve(udp) }()
	t.Cleanup(func() { _ = wt.Close(); _ = udp.Close(); <-done })

	for _, flowControl := range []bool{false, true} {
		name := "legacy client"
		if flowControl {
			name = "session flow control"
		}
		t.Run(name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			client := webtransport.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, //nolint:gosec // generated loopback test cert
			}
			if flowControl {
				client.Config = &webtransport.Config{
					MaxIncomingStreams: 16, MaxIncomingUniStreams: 16, MaxIncomingData: 64 << 10,
				}
			}
			_, session, err := client.Dial(ctx, "https://"+udp.LocalAddr().String()+"/wt", nil)
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = session.CloseWithError(0, "test done") }()
			// Dial completed server initialization. These settings must all be
			// present and positive for Safari to create a stream.
			for _, id := range []uint64{0x2b65, 0x2b64, 0x2b61} {
				if wt.H3.AdditionalSettings[id] == 0 {
					t.Fatalf("missing initial WebTransport credit setting %#x", id)
				}
			}
			stream, err := session.OpenStreamSync(ctx)
			if err != nil {
				t.Fatal(err)
			}
			deadline, _ := ctx.Deadline()
			if err := stream.SetDeadline(deadline); err != nil {
				t.Fatal(err)
			}
			payload := bytes.Repeat([]byte("terminal data\n"), 256*1024) // > 3 MiB, larger than either window
			written := make(chan error, 1)
			go func() {
				_, err := stream.Write(payload)
				written <- err
			}()
			echoed := make([]byte, len(payload))
			if _, err := io.ReadFull(stream, echoed); err != nil {
				t.Fatal(err)
			}
			if err := <-written; err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(echoed, payload) {
				t.Fatal("echo data differs")
			}
		})
	}
}
