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
	dur := flag.Float64("dur", 7.0, "seconds a label takes to cross the screen")
	lanes := flag.Int("lanes", 0, "number of horizontal lanes (0 = fit to height)")
	topmost := flag.Bool("topmost", true, "keep the strip above other windows")
	noActivate := flag.Bool("no-activate", false, "never take keyboard focus")
	debug := flag.Bool("debug", false, "enable devtools and the context menu")
	ignoreSynthetic := flag.Bool("ignore-injected", false, "ignore synthetic keystrokes from macros or remote sessions")
	flag.Parse()

	ignoreInjected = *ignoreSynthetic

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

	addr, err := serveAssets()
	if err != nil {
		log.Fatalf("http: %v", err)
	}

	url := fmt.Sprintf("http://%s/index.html?dur=%g", addr, *dur)
	if *lanes > 0 {
		url += fmt.Sprintf("&lanes=%d", *lanes)
	}

	w.Navigate(url)
	w.Run()
}

// push marshals one event and hands it to the page. Dispatch is mandatory:
// Eval and Navigate are WebView2 COM calls and must run on the UI thread.
func push(w webview2.WebView, ev Event) {
	payload, err := json.Marshal(ev)
	if err != nil {
		return
	}
	script := fmt.Sprintf("window.__hud&&window.__hud(%s)", payload)
	w.Dispatch(func() { w.Eval(script) })
}

// serveAssets exposes the embedded UI on a loopback port. A real origin keeps
// web fonts and modules behaving exactly as they do in a normal browser.
func serveAssets() (string, error) {
	sub, err := fs.Sub(webAssets, "web")
	if err != nil {
		return "", err
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", err
	}
	srv := &http.Server{Handler: http.FileServer(http.FS(sub))}
	go func() {
		if err := srv.Serve(ln); err != nil && err != http.ErrServerClosed {
			log.Printf("http server: %v", err)
		}
	}()
	return ln.Addr().String(), nil
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
