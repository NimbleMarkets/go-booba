//go:build !js

package serve

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"embed"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net"
	"net/http"
	"net/url"
	"path"
	"sync/atomic"
	"time"

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
	newSession  SessionFactory
}

// NewServer creates a new server with the given config.
func NewServer(config Config) *Server {
	return &Server{
		config:     config,
		newSession: defaultSessionFactory,
	}
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

// SetSessionFactory overrides how terminal sessions are created.
func (s *Server) SetSessionFactory(factory SessionFactory) {
	if factory == nil {
		s.newSession = defaultSessionFactory
		return
	}
	s.newSession = factory
}

// HTTPHandler constructs the application HTTP handler without starting listeners.
func (s *Server) HTTPHandler() (http.Handler, error) {
	return s.newMux(nil)
}

func (s *Server) start(ctx context.Context) error {
	if err := s.configureTransport(); err != nil {
		return err
	}

	wtServer := s.newWebTransportServer()
	mux, err := s.newMux(wtServer)
	if err != nil {
		return err
	}

	if wtServer != nil {
		s.startWebTransport(ctx, wtServer)
	}

	addr := fmt.Sprintf("%s:%d", s.config.Host, s.config.Port)
	logHostName := s.config.Host
	if logHostName == "" {
		logHostName = "localhost"
	}
	log.Printf("Starting server on %s://%s:%d", s.httpScheme(), logHostName, s.config.Port)

	server := &http.Server{
		Addr:        addr,
		Handler:     mux,
		IdleTimeout: s.config.IdleTimeout,
		BaseContext: func(l net.Listener) context.Context {
			return ctx
		},
	}

	// Graceful shutdown
	go func() {
		<-ctx.Done()
		server.Close()
	}()

	return s.listenAndServeHTTP(server)
}

func (s *Server) configureTransport() error {
	if s.config.CertFile != "" && s.config.KeyFile != "" {
		cert, err := tls.LoadX509KeyPair(s.config.CertFile, s.config.KeyFile)
		if err != nil {
			return fmt.Errorf("load TLS cert: %w", err)
		}
		s.certInfo = newCertInfo(cert)
		return nil
	}

	host := s.config.Host
	if host == "" || host == "0.0.0.0" {
		host = "localhost"
	}
	certInfo, err := GenerateSelfSignedCert(host)
	if err != nil {
		log.Printf("WebTransport disabled: cert generation failed: %v", err)
		s.certInfo = nil
		return nil
	}

	s.certInfo = certInfo
	s.debugf("Generated self-signed cert (hash: %s)", hex.EncodeToString(s.certInfo.Hash[:]))
	return nil
}

func (s *Server) newMux(wtServer *webtransport.Server) (http.Handler, error) {
	mux := http.NewServeMux()

	staticFS, err := fs.Sub(staticFiles, "static")
	if err != nil {
		return nil, fmt.Errorf("static fs: %w", err)
	}
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.FS(staticFS))))
	mux.HandleFunc("/", s.handleIndex)
	mux.HandleFunc("/ws", s.handleWS)

	if s.certInfo != nil {
		mux.HandleFunc("/cert-hash", s.handleCertHash)
	}
	if wtServer != nil {
		mux.HandleFunc("/wt", func(w http.ResponseWriter, r *http.Request) {
			s.debugf("WT handler: %s %s %s proto=%s", r.Method, r.URL.Path, r.URL.String(), r.Proto)
			s.handleWT(w, r, wtServer)
		})
	}

	return mux, nil
}

func (s *Server) newWebTransportServer() *webtransport.Server {
	http3Port := s.config.HTTP3Port
	if http3Port == 0 {
		http3Port = s.config.Port
	}
	if s.certInfo == nil || http3Port < 0 {
		return nil
	}

	wtAddr := fmt.Sprintf("%s:%d", s.config.Host, http3Port)
	wtServer := &webtransport.Server{
		H3: &http3.Server{
			Addr:            wtAddr,
			TLSConfig:       s.http3TLSConfig(),
			EnableDatagrams: true,
		},
		CheckOrigin: s.checkOrigin,
	}
	webtransport.ConfigureHTTP3Server(wtServer.H3)
	return wtServer
}

func (s *Server) startWebTransport(ctx context.Context, wtServer *webtransport.Server) {
	go func() {
		logHostName := s.config.Host
		if logHostName == "" {
			logHostName = "localhost"
		}
		_, port, err := net.SplitHostPort(wtServer.H3.Addr)
		if err != nil {
			port = wtServer.H3.Addr
		}
		log.Printf("WebTransport listening on https://%s:%s", logHostName, port)
		if err := wtServer.ListenAndServe(); err != nil {
			log.Printf("WebTransport server error: %v", err)
		}
	}()

	go func() {
		<-ctx.Done()
		wtServer.Close()
	}()
}

func (s *Server) listenAndServeHTTP(server *http.Server) error {
	if s.mainTLSEnabled() {
		server.TLSConfig = s.httpsTLSConfig()
		return server.ListenAndServeTLS("", "")
	}
	return server.ListenAndServe()
}

func newCertInfo(cert tls.Certificate) *CertInfo {
	info := &CertInfo{Certificate: cert}
	if len(cert.Certificate) > 0 {
		info.DER = cert.Certificate[0]
		info.Hash = sha256.Sum256(info.DER)
	}
	return info
}

func (s *Server) mainTLSEnabled() bool {
	return s.config.CertFile != "" && s.config.KeyFile != ""
}

func (s *Server) httpScheme() string {
	if s.mainTLSEnabled() {
		return "https"
	}
	return "http"
}

func (s *Server) httpsTLSConfig() *tls.Config {
	if s.certInfo == nil {
		return nil
	}
	return &tls.Config{
		Certificates: []tls.Certificate{s.certInfo.Certificate},
		MinVersion:   tls.VersionTLS12,
		NextProtos:   []string{"h2", "http/1.1"},
	}
}

func (s *Server) http3TLSConfig() *tls.Config {
	if s.certInfo == nil {
		return nil
	}
	return &tls.Config{
		Certificates: []tls.Certificate{s.certInfo.Certificate},
		MinVersion:   tls.VersionTLS13,
		NextProtos:   []string{"h3"},
	}
}

func (s *Server) tryAcquireConnection() bool {
	if s.config.MaxConnections <= 0 {
		s.connCount.Add(1)
		return true
	}

	for {
		current := s.connCount.Load()
		if int(current) >= s.config.MaxConnections {
			return false
		}
		if s.connCount.CompareAndSwap(current, current+1) {
			return true
		}
	}
}

func (s *Server) releaseConnection() {
	s.connCount.Add(-1)
}

func (s *Server) debugf(format string, args ...any) {
	if s.config.Debug {
		log.Printf(format, args...)
	}
}

func (s *Server) attachIdleTimeout(ctx context.Context, sess Session) (context.Context, context.CancelFunc, func()) {
	if s.config.IdleTimeout <= 0 {
		return ctx, func() {}, func() {}
	}

	idleCtx, cancel := context.WithCancel(ctx)
	activityCh := make(chan struct{}, 1)

	go func() {
		timer := time.NewTimer(s.config.IdleTimeout)
		defer timer.Stop()

		for {
			select {
			case <-idleCtx.Done():
				return
			case <-activityCh:
				if !timer.Stop() {
					select {
					case <-timer.C:
					default:
					}
				}
				timer.Reset(s.config.IdleTimeout)
			case <-timer.C:
				s.debugf("Closing idle session after %s", s.config.IdleTimeout)
				cancel()
				if err := sess.Close(); err != nil && !errors.Is(err, io.EOF) {
					log.Printf("session close error: %v", err)
				}
				return
			}
		}
	}()

	activity := func() {
		select {
		case activityCh <- struct{}{}:
		default:
		}
	}

	return idleCtx, cancel, activity
}

func (s *Server) createSession(ctx context.Context, size WindowSize) (Session, error) {
	factory := s.newSession
	if factory == nil {
		factory = defaultSessionFactory
	}
	return factory(ctx, size)
}

func (s *Server) runSession(ctx context.Context, sess Session) error {
	switch {
	case s.handler != nil:
		return runBubbleTea(ctx, sess, s.handler, nil)
	case s.progHandler != nil:
		return runBubbleTeaProgram(ctx, sess, s.progHandler)
	case s.cmdName != "":
		ptySess, ok := sess.(*ptySession)
		if !ok {
			return fmt.Errorf("command mode requires PTY session")
		}
		return runCommand(ctx, ptySess, s.cmdName, s.cmdArgs...)
	default:
		select {
		case <-ctx.Done():
			return nil
		case <-sess.Done():
			return nil
		}
	}
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
	if _, err := w.Write(data); err != nil {
		log.Printf("write index response: %v", err)
	}
}

func (s *Server) handleWS(w http.ResponseWriter, r *http.Request) {
	if !s.checkAuth(w, r) {
		return
	}
	if !s.tryAcquireConnection() {
		http.Error(w, "max connections reached", http.StatusServiceUnavailable)
		return
	}
	defer s.releaseConnection()

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
		_ = conn.CloseNow()
		return
	}
	msgType, payload, err := DecodeWSMessage(data)
	if err != nil || msgType != MsgResize {
		_ = conn.CloseNow()
		return
	}
	var rm ResizeMessage
	if err := json.Unmarshal(payload, &rm); err != nil || rm.Cols <= 0 || rm.Rows <= 0 {
		_ = conn.CloseNow()
		return
	}

	// Create PTY session
	sess, err := s.createSession(ctx, WindowSize{Width: rm.Cols, Height: rm.Rows})
	if err != nil {
		log.Printf("create session: %v", err)
		_ = conn.CloseNow()
		return
	}
	defer sess.Close()

	ctx, cancel, activity := s.attachIdleTimeout(ctx, sess)
	defer cancel()
	activity()

	s.debugf("New session: %dx%d", rm.Cols, rm.Rows)

	opts := OptionsMessage{ReadOnly: s.config.ReadOnly}

	// Start the session workload in a goroutine
	go func() {
		defer sess.Close()
		if err := s.runSession(ctx, sess); err != nil {
			log.Printf("session error: %v", err)
		}
	}()

	// Handle WebSocket protocol messages (blocks until disconnect)
	handleWebSocket(ctx, conn, sess, opts, s.config.Debug, activity)
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
	if err := json.NewEncoder(w).Encode(map[string]string{
		"hash": hex.EncodeToString(s.certInfo.Hash[:]),
	}); err != nil {
		log.Printf("encode cert hash response: %v", err)
	}
}

func (s *Server) handleWT(w http.ResponseWriter, r *http.Request, wtServer *webtransport.Server) {
	s.debugf("WebTransport handler called: %s %s %s", r.Method, r.URL.Path, r.Proto)
	if !s.checkAuth(w, r) {
		return
	}
	if !s.tryAcquireConnection() {
		http.Error(w, "max connections reached", http.StatusServiceUnavailable)
		return
	}
	defer s.releaseConnection()

	wtSess, err := wtServer.Upgrade(w, r)
	if err != nil {
		log.Printf("webtransport upgrade: %v", err)
		return
	}
	defer func() {
		if err := wtSess.CloseWithError(0, ""); err != nil {
			log.Printf("webtransport session close: %v", err)
		}
	}()

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

	sess, err := s.createSession(ctx, WindowSize{Width: rm.Cols, Height: rm.Rows})
	if err != nil {
		log.Printf("create session: %v", err)
		return
	}
	defer sess.Close()

	ctx, cancel, activity := s.attachIdleTimeout(ctx, sess)
	defer cancel()
	activity()

	s.debugf("New WebTransport session: %dx%d", rm.Cols, rm.Rows)

	opts := OptionsMessage{ReadOnly: s.config.ReadOnly}

	go func() {
		defer sess.Close()
		if err := s.runSession(ctx, sess); err != nil {
			log.Printf("session error: %v", err)
		}
	}()

	handleWebTransport(ctx, sess, stream, opts, s.config.Debug, activity)
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
