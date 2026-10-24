// Command key-hud is a decorative fullscreen strip that shows every key and
// mouse button you press, anywhere in Windows, as pixel text that sweeps
// right-to-left and fades out.
//
// It is designed to fill a 1920x480 secondary display. Exit with Ctrl+Alt+Q.
package main

import (
	"embed"
	"encoding/json"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	webview2 "github.com/jchv/go-webview2"
)

//go:embed web
var webAssets embed.FS

func main() {
	monitorSpec := flag.String("monitor", "1920x480", `target screen: "WxH", a 1-based index, or "primary"`)
	configFile := flag.String("config", "", "path to key-hud.json (default: next to the executable)")
	topmost := flag.Bool("topmost", true, "keep the strip above other windows")
	noActivate := flag.Bool("no-activate", false, "never take keyboard focus")
	debug := flag.Bool("debug", false, "enable devtools and the context menu")
	ignoreSynthetic := flag.Bool("ignore-injected", false, "ignore synthetic keystrokes from macros or remote sessions")
	flag.Parse()

	ignoreInjected = *ignoreSynthetic

	if !acquireSingleInstance() {
		log.Print("key-hud is already running; this second copy is exiting")
		return
	}

	cfgPath := *configFile
	if cfgPath == "" {
		cfgPath = configPath()
	}
	cfg := loadConfig(cfgPath)

	target, err := pickMonitor(*monitorSpec)
	if err != nil {
		log.Fatalf("monitor: %v", err)
	}
	log.Printf("target screen: %dx%d at (%d,%d)", target.W(), target.H(), target.Left, target.Top)

	w := webview2.NewWithOptions(webview2.WebViewOptions{
		Debug:     *debug,
		DataPath:  webviewDataPath(),
		AutoFocus: false,
		WindowOptions: webview2.WindowOptions{
			Title:  "key-hud",
			Width:  uint(target.W()),
			Height: uint(target.H()),
		},
	})
	if w == nil {
		log.Fatal("could not create the window: is the WebView2 runtime installed?")
	}
	defer w.Destroy()

	styleAsOverlay(uintptr(w.Window()), target, *topmost, *noActivate)

	grouperRef = newGrouper(time.Duration(cfg.GroupWindowMs)*time.Millisecond, cfg.Lanes)

	quit := make(chan struct{})
	var startOnce sync.Once

	// Hooks only start once the page exists, so no keystroke is lost into an
	// empty document. A timer backs the ready signal up in case the binding
	// ever fails.
	startHooksOnce := func() {
		startOnce.Do(func() {
			events := startHooks(quit)
			go func() {
				for ev := range events {
					push(w, ev)
				}
			}()
			log.Print("global hooks installed")
		})
	}

	if err := w.Bind("hudReady", func() (string, error) {
		startHooksOnce()
		return "ok", nil
	}); err != nil {
		log.Printf("bind hudReady: %v (falling back to timer)", err)
	}

	go func() {
		select {
		case <-time.After(2 * time.Second):
			startHooksOnce()
		case <-quit:
		}
	}()

	go func() {
		<-quit
		w.Dispatch(func() { w.Terminate() })
	}()

	addr, err := serveAssets(cfg)
	if err != nil {
		log.Fatalf("http: %v", err)
	}

	w.Navigate(fmt.Sprintf("http://%s/index.html", addr))
	w.Run()
}

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
