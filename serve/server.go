//go:build !js

package serve

import (
	"context"
	"embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"log"
	"net"
	"net/http"
	"sync/atomic"

	"github.com/coder/websocket"
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

	addr := fmt.Sprintf("%s:%d", s.config.Host, s.config.Port)
	log.Printf("Starting server on http://%s", addr)

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
		InsecureSkipVerify: true,
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
	if s.certInfo == nil {
		http.Error(w, "no certificate", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"hash": hex.EncodeToString(s.certInfo.Hash[:]),
	})
}
