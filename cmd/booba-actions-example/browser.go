//go:build !js

package main

import (
	_ "embed"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/NimbleMarkets/go-booba/serve"
)

//go:embed browser.html
var browserHTML []byte

func browserHandler(ghosttyDir string) (http.Handler, error) {
	config := serve.DefaultConfig()
	config.HTTP3Port = -1
	server := serve.NewServer(config)
	base, err := server.HTTPHandlerFor(func(sess serve.Session) (tea.Model, []tea.ProgramOption) {
		return newActionModel(), nil
	})
	if err != nil {
		return nil, err
	}
	mux := http.NewServeMux()
	if ghosttyDir != "" {
		for _, name := range []string{"ghostty-web.js", "ghostty-vt.wasm"} {
			if _, err := os.Stat(filepath.Join(ghosttyDir, name)); err != nil {
				return nil, err
			}
		}
		mux.Handle("/static/ghostty-web/", http.StripPrefix("/static/ghostty-web/", http.FileServer(http.Dir(ghosttyDir))))
	}
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			base.ServeHTTP(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(browserHTML)
	})
	return mux, nil
}
func serveBrowser(address, ghosttyDir string) error {
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return err
	}
	ip := net.ParseIP(host)
	if host != "localhost" && (ip == nil || !ip.IsLoopback()) {
		return fmt.Errorf("browser demo must listen on a loopback address")
	}
	handler, err := browserHandler(ghosttyDir)
	if err != nil {
		return err
	}
	log.Printf("Terminal Actions demo: http://%s", address)
	server := &http.Server{Addr: address, Handler: handler, ReadHeaderTimeout: 5 * time.Second}
	return server.ListenAndServe()
}
