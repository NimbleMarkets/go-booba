//go:build !js

package serve

import (
	"context"
	"crypto/tls"
	"embed"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net"
	"net/http"
	"net/url"
	"path"
	"sync/atomic"

	"github.com/coder/websocket"
	"github.com/quic-go/quic-go/http3"
	"github.com/quic-go/webtransport-go"
)

//go:embed static/*
var staticFiles embed.FS

// Server serves terminal sessions over WebSocket.
type Server struct {
	config      Config
	handler     Handler
	progHandler ProgramHandler
	cmdName     string
	cmdArgs     []string
	connCount   atomic.Int32
	certInfo    *CertInfo
}

// NewServer creates a new server with the given config.
func NewServer(config Config) *Server {
	return &Server{config: config}
}

// Serve starts the server with a BubbleTea handler.
func (s *Server) Serve(ctx context.Context, handler Handler) error {
	s.handler = handler
	return s.start(ctx)
}

// ServeWithProgram starts the server with a ProgramHandler.
func (s *Server) ServeWithProgram(ctx context.Context, handler ProgramHandler) error {
	s.progHandler = handler
	return s.start(ctx)
}

// ServeCommand starts the server wrapping an external command.
func (s *Server) ServeCommand(ctx context.Context, name string, args ...string) error {
	s.cmdName = name
	s.cmdArgs = args
	return s.start(ctx)
}

func (s *Server) start(ctx context.Context) error {
	// Set up TLS for WebTransport
	if s.config.TLSCert != "" && s.config.TLSKey != "" {
		cert, err := tls.LoadX509KeyPair(s.config.TLSCert, s.config.TLSKey)
		if err != nil {
			return fmt.Errorf("load TLS cert: %w", err)
		}
		s.certInfo = &CertInfo{
			TLSConfig: &tls.Config{
				Certificates: []tls.Certificate{cert},
				NextProtos:   []string{"h3"},
			},
		}
	} else {
		host := s.config.Host
		if host == "" || host == "0.0.0.0" {
			host = "localhost"
		}
		certInfo, err := GenerateSelfSignedCert(host)
		if err != nil {
			log.Printf("WebTransport disabled: cert generation failed: %v", err)
		} else {
			s.certInfo = certInfo
			log.Printf("Generated self-signed cert (hash: %s)", hex.EncodeToString(s.certInfo.Hash[:]))
		}
	}

	mux := http.NewServeMux()

	// Static files (ghostty-web assets, compiled TypeScript)
	staticFS, err := fs.Sub(staticFiles, "static")
	if err != nil {
		return fmt.Errorf("static fs: %w", err)
	}
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.FS(staticFS))))
	mux.HandleFunc("/", s.handleIndex)

	// WebSocket endpoint
	mux.HandleFunc("/ws", s.handleWS)

	// Certificate hash endpoint for WebTransport
	if s.certInfo != nil {
		mux.HandleFunc("/cert-hash", s.handleCertHash)
	}

	// Start WebTransport server
	wtPort := s.config.WTPort
	if wtPort == 0 {
		wtPort = s.config.Port + 1
	}
	if s.certInfo != nil && wtPort > 0 {
		wtAddr := fmt.Sprintf("%s:%d", s.config.Host, wtPort)
		wtServer := &webtransport.Server{
			H3: &http3.Server{
				Addr:            wtAddr,
				TLSConfig:       s.certInfo.TLSConfig,
				EnableDatagrams: true,
			},
			CheckOrigin: s.checkOrigin,
		}
		webtransport.ConfigureHTTP3Server(wtServer.H3)

		wtServer.H3.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			log.Printf("WT handler: %s %s %s proto=%s", r.Method, r.URL.Path, r.URL.String(), r.Proto)
			s.handleWT(w, r, wtServer)
		})

		go func() {
			logHostName := s.config.Host
			if logHostName == "" {
				logHostName = "localhost"
			}
			log.Printf("WebTransport listening on https://%s:%d", logHostName, wtPort)
			if err := wtServer.ListenAndServe(); err != nil {
				log.Printf("WebTransport server error: %v", err)
			}
		}()

		// Graceful shutdown
		go func() {
			<-ctx.Done()
			wtServer.Close()
		}()
	}

	addr := fmt.Sprintf("%s:%d", s.config.Host, s.config.Port)
	logHostName := s.config.Host
	if logHostName == "" {
		logHostName = "localhost"
	}
	log.Printf("Starting server on http://%s:%d", logHostName, s.config.Port)

	server := &http.Server{
		Addr:    addr,
		Handler: mux,
		BaseContext: func(l net.Listener) context.Context {
			return ctx
		},
	}

	// Graceful shutdown
	go func() {
		<-ctx.Done()
		server.Close()
	}()

	return server.ListenAndServe()
}

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	if !s.checkAuth(w, r) {
		return
	}
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	data, err := staticFiles.ReadFile("static/index.html")
	if err != nil {
		http.Error(w, "index not found", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write(data)
}

func (s *Server) handleWS(w http.ResponseWriter, r *http.Request) {
	if !s.checkAuth(w, r) {
		return
	}
	// Connection limit check
	if s.config.MaxConnections > 0 {
		if int(s.connCount.Load()) >= s.config.MaxConnections {
			http.Error(w, "max connections reached", http.StatusServiceUnavailable)
			return
		}
	}
	s.connCount.Add(1)
	defer s.connCount.Add(-1)

	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		OriginPatterns: s.config.OriginPatterns,
	})
	if err != nil {
		log.Printf("websocket accept: %v", err)
		return
	}
	conn.SetReadLimit(MaxMessageSize)

	ctx := r.Context()

	// Wait for initial resize from client
	_, data, err := conn.Read(ctx)
	if err != nil {
		conn.CloseNow()
		return
	}
	msgType, payload, err := DecodeWSMessage(data)
	if err != nil || msgType != MsgResize {
		conn.CloseNow()
		return
	}
	var rm ResizeMessage
	if err := json.Unmarshal(payload, &rm); err != nil || rm.Cols <= 0 || rm.Rows <= 0 {
		conn.CloseNow()
		return
	}

	// Create PTY session
	sess, err := newPtySession(ctx, WindowSize{Width: rm.Cols, Height: rm.Rows})
	if err != nil {
		log.Printf("create session: %v", err)
		conn.CloseNow()
		return
	}
	defer sess.Close()

	log.Printf("New session: %dx%d", rm.Cols, rm.Rows)

	opts := OptionsMessage{ReadOnly: s.config.ReadOnly}

	// Start the session workload in a goroutine
	go func() {
		defer sess.Close()
		var runErr error
		switch {
		case s.handler != nil:
			runErr = runBubbleTea(ctx, sess, s.handler, nil)
		case s.progHandler != nil:
			runErr = runBubbleTeaProgram(ctx, sess, s.progHandler)
		case s.cmdName != "":
			runErr = runCommand(ctx, sess, s.cmdName, s.cmdArgs...)
		}
		if runErr != nil {
			log.Printf("session error: %v", runErr)
		}
	}()

	// Handle WebSocket protocol messages (blocks until disconnect)
	handleWebSocket(ctx, conn, sess, opts)
}

func (s *Server) handleCertHash(w http.ResponseWriter, r *http.Request) {
	if !s.checkAuth(w, r) {
		return
	}
	if s.certInfo == nil {
		http.Error(w, "no certificate", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"hash": hex.EncodeToString(s.certInfo.Hash[:]),
	})
}

func (s *Server) handleWT(w http.ResponseWriter, r *http.Request, wtServer *webtransport.Server) {
	log.Printf("WebTransport handler called: %s %s %s", r.Method, r.URL.Path, r.Proto)
	if !s.checkAuth(w, r) {
		return
	}
	if s.config.MaxConnections > 0 {
		if int(s.connCount.Load()) >= s.config.MaxConnections {
			http.Error(w, "max connections reached", http.StatusServiceUnavailable)
			return
		}
	}
	s.connCount.Add(1)
	defer s.connCount.Add(-1)

	wtSess, err := wtServer.Upgrade(w, r)
	if err != nil {
		log.Printf("webtransport upgrade: %v", err)
		return
	}
	defer wtSess.CloseWithError(0, "")

	stream, err := wtSess.AcceptStream(r.Context())
	if err != nil {
		log.Printf("webtransport accept stream: %v", err)
		return
	}

	ctx := r.Context()

	// Read initial resize (length-prefixed)
	lenBuf := make([]byte, 4)
	if _, err := io.ReadFull(stream, lenBuf); err != nil {
		return
	}
	msgLen := binary.BigEndian.Uint32(lenBuf)
	if msgLen == 0 || msgLen > MaxMessageSize {
		return
	}
	msgBuf := make([]byte, msgLen)
	if _, err := io.ReadFull(stream, msgBuf); err != nil {
		return
	}
	if msgBuf[0] != MsgResize {
		return
	}
	var rm ResizeMessage
	if err := json.Unmarshal(msgBuf[1:], &rm); err != nil || rm.Cols <= 0 || rm.Rows <= 0 {
		return
	}

	sess, err := newPtySession(ctx, WindowSize{Width: rm.Cols, Height: rm.Rows})
	if err != nil {
		log.Printf("create session: %v", err)
		return
	}
	defer sess.Close()

	log.Printf("New WebTransport session: %dx%d", rm.Cols, rm.Rows)

	opts := OptionsMessage{ReadOnly: s.config.ReadOnly}

	go func() {
		defer sess.Close()
		var runErr error
		switch {
		case s.handler != nil:
			runErr = runBubbleTea(ctx, sess, s.handler, nil)
		case s.progHandler != nil:
			runErr = runBubbleTeaProgram(ctx, sess, s.progHandler)
		case s.cmdName != "":
			runErr = runCommand(ctx, sess, s.cmdName, s.cmdArgs...)
		}
		if runErr != nil {
			log.Printf("session error: %v", runErr)
		}
	}()

	handleWebTransport(ctx, sess, stream, opts)
}

func (s *Server) checkAuth(w http.ResponseWriter, r *http.Request) bool {
	if s.config.BasicUsername == "" && s.config.BasicPassword == "" {
		return true
	}

	username, password, ok := r.BasicAuth()
	if !ok || username != s.config.BasicUsername || password != s.config.BasicPassword {
		w.Header().Set("WWW-Authenticate", `Basic realm="booba"`)
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return false
	}
	return true
}

func (s *Server) checkOrigin(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return true
	}

	parsed, err := url.Parse(origin)
	if err != nil || parsed.Host == "" {
		return false
	}

	if sameOriginHost(parsed.Hostname(), r.Host) {
		return true
	}

	candidate := parsed.Host
	if parsed.Scheme != "" {
		candidate = parsed.Scheme + "://" + parsed.Host
	}

	for _, patternValue := range s.config.OriginPatterns {
		ok, err := path.Match(patternValue, candidate)
		if err == nil && ok {
			return true
		}
		ok, err = path.Match(patternValue, parsed.Host)
		if err == nil && ok {
			return true
		}
	}

	return false
}

func sameOriginHost(originHost, requestHost string) bool {
	if originHost == "" || requestHost == "" {
		return false
	}

	requestParsed := requestHost
	if host, _, err := net.SplitHostPort(requestHost); err == nil {
		requestParsed = host
	}

	return originHost == requestParsed
}
