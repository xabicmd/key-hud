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

	"keyhud/internal/capture"
	"keyhud/internal/config"
	"keyhud/internal/platform"
	"keyhud/internal/stage"
)

func main() {
	monitorSpec := flag.String("monitor", "1920x480", `target screen: "WxH", a 1-based index, or "primary"`)
	configFile := flag.String("config", "", "path to key-hud.json (default: next to the executable)")
	topmost := flag.Bool("topmost", true, "keep the strip above other windows")
	noActivate := flag.Bool("no-activate", false, "never take keyboard focus")
	debug := flag.Bool("debug", false, "enable devtools and the context menu")
	ignoreSynthetic := flag.Bool("ignore-injected", false, "ignore synthetic keystrokes from macros or remote sessions")
	flag.Parse()

	if !platform.AcquireSingleInstance() {
		log.Print("key-hud is already running; this second copy is exiting")
		return
	}

	cfgPath := *configFile
	if cfgPath == "" {
		cfgPath = config.Path()
	}
	cfg := config.Load(cfgPath)

	target, err := platform.PickMonitor(*monitorSpec)
	if err != nil {
		log.Fatalf("monitor: %v", err)
	}
	log.Printf("target screen: %dx%d at (%d,%d)", target.W(), target.H(), target.Left, target.Top)

	w := webview2.NewWithOptions(webview2.WebViewOptions{
		Debug:     *debug,
		DataPath:  stage.WebviewDataPath(),
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

	platform.StyleAsOverlay(uintptr(w.Window()), target, *topmost, *noActivate)

	// The intro comes up before the hooks exist, so the strip is muted from the
	// first key. gate.go owns the rest.
	gate := capture.NewGate()
	input := capture.New(capture.Options{
		Gate:           gate,
		IgnoreInjected: *ignoreSynthetic,
		GroupWindow:    time.Duration(cfg.GroupWindowMs) * time.Millisecond,
		Lanes:          cfg.Lanes,
	})

	quit := make(chan struct{})
	var startOnce sync.Once

	// Hooks only start once the page exists, so no keystroke is lost into an
	// empty document. A timer backs the ready signal up in case the binding
	// ever fails.
	startHooksOnce := func() {
		startOnce.Do(func() {
			events := input.Start(quit)
			go func() {
				for ev := range events {
					stage.Push(w, ev)
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
		return gate.Pending(), nil
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
		gate.WaitStart()
		log.Print("intro dismissed; the strip is live")
		stage.HideIntro(w)
	}()

	addr, err := stage.ServeAssets(cfg)
	if err != nil {
		log.Fatalf("http: %v", err)
	}

	w.Navigate(fmt.Sprintf("http://%s/index.html", addr))
	w.Run()
}
