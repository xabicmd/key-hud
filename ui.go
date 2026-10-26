package main

import (
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"

	webview2 "github.com/jchv/go-webview2"
)

// This file is the whole boundary between Go and the page: the embedded
// assets, the loopback origin that serves them, and the one-way transport used
// to push instructions into the document.

//go:embed web
var webAssets embed.FS

// push marshals one instruction and hands it to the page. Dispatch is
// mandatory: Eval and Navigate are WebView2 COM calls and must run on the UI
// thread.
func push(w webview2.WebView, ev Event) {
	payload, err := json.Marshal(ev)
	if err != nil {
		return
	}
	script := fmt.Sprintf("window.__hud&&window.__hud(%s)", payload)
	w.Dispatch(func() { w.Eval(script) })
}

// serveAssets exposes the embedded UI plus the live config on a loopback port.
// A real origin keeps web fonts and modules behaving exactly as they do in a
// normal browser.
func serveAssets(cfg Config) (string, error) {
	sub, err := fs.Sub(webAssets, "web")
	if err != nil {
		return "", err
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/api/config", func(rw http.ResponseWriter, r *http.Request) {
		rw.Header().Set("Content-Type", "application/json")
		rw.Header().Set("Cache-Control", "no-store")
		if err := json.NewEncoder(rw).Encode(cfg); err != nil {
			log.Printf("config endpoint: %v", err)
		}
	})
	mux.Handle("/", noCache(http.FileServer(http.FS(sub))))

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", err
	}
	srv := &http.Server{Handler: mux}
	go func() {
		if err := srv.Serve(ln); err != nil && err != http.ErrServerClosed {
			log.Printf("http server: %v", err)
		}
	}()
	return ln.Addr().String(), nil
}

// noCache keeps WebView2 from serving a stale style.css or app.js after a
// rebuild. The port is random but frequently reused, so the origin can match a
// previous run and Chromium happily reuses its cached copies. That is exactly
// how an old animation once kept winning over the new one.
func noCache(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store, must-revalidate")
		w.Header().Set("Pragma", "no-cache")
		w.Header().Set("Expires", "0")
		h.ServeHTTP(w, r)
	})
}

func webviewDataPath() string {
	base, err := os.UserCacheDir()
	if err != nil || base == "" {
		return ""
	}
	p := filepath.Join(base, "key-hud", "webview2")
	if err := os.MkdirAll(p, 0o755); err != nil {
		return ""
	}
	return p
}
