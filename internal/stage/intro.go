package stage

import (
	webview2 "github.com/jchv/go-webview2"
)

// HideIntro asks the page to drop the intro and start the strip.
//
// It lives here, not in capture, because it is a WebView2 call: Eval must run
// on the UI thread, and the input pipeline must not know the window exists.
func HideIntro(w webview2.WebView) {
	w.Dispatch(func() { w.Eval("window.__hudStart&&window.__hudStart()") })
}
