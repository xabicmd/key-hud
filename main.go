// Command key-hud is a decorative fullscreen strip that shows every key and
// mouse button you press, anywhere in Windows, as pixel text that sweeps
// right-to-left and fades out.
//
// It is designed to fill a 1920x480 secondary display. Press Enter on the intro
// screen to start it, and Ctrl+Alt+Q to exit.
package main

import (
	"flag"
	"fmt"
	"log"
	"sync"
	"time"

	webview2 "github.com/jchv/go-webview2"
)

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

	// The intro comes up before the hooks exist, so the strip is muted from the
	// first key. splash.go owns the rest.
	splash := newSplashGate()
	splashRef = splash

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

	// The page asks once at boot whether the intro is still up: the Enter press
	// can land before the document exists, and the push would be lost.
	if err := w.Bind("hudIntro", func() (bool, error) {
		return splash.Pending(), nil
	}); err != nil {
		log.Printf("bind hudIntro: %v", err)
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

	go func() {
		splash.waitStart()
		log.Print("intro dismissed; the strip is live")
		hideIntro(w)
	}()

	addr, err := serveAssets(cfg)
	if err != nil {
		log.Fatalf("http: %v", err)
	}

	w.Navigate(fmt.Sprintf("http://%s/index.html", addr))
	w.Run()
}
