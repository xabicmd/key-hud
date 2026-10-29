package stage

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"keyhud/internal/config"
)

// The origin is the whole contract with the page: the embedded assets, the
// config endpoint and the no-cache headers. A wrong embed root would only show
// up as a blank window, so it is pinned here.
func TestServeAssetsOriginAndConfigShape(t *testing.T) {
	addr, err := ServeAssets(config.Default())
	if err != nil {
		t.Fatalf("ServeAssets: %v", err)
	}
	if !strings.HasPrefix(addr, "127.0.0.1:") {
		t.Fatalf("addr = %q, want a loopback origin", addr)
	}

	resp, err := http.Get("http://" + addr + "/api/config")
	if err != nil {
		t.Fatalf("GET /api/config: %v", err)
	}
	defer resp.Body.Close()
	if cc := resp.Header.Get("Cache-Control"); cc != "no-store" {
		t.Errorf("/api/config Cache-Control = %q", cc)
	}
	var got map[string]json.RawMessage
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatalf("decode /api/config: %v", err)
	}
	for _, key := range []string{"speedPxPerSec", "groupWindowMs", "lanes", "fontSizeVh", "travelEndPx", "colors"} {
		if _, ok := got[key]; !ok {
			t.Errorf("/api/config is missing %q", key)
		}
	}

	for _, path := range []string{"/", "/index.html", "/app.js", "/config.js", "/motion.js", "/splash.js", "/stage.js", "/style.css", "/splash.css"} {
		r, err := http.Get("http://" + addr + path)
		if err != nil {
			t.Fatalf("GET %s: %v", path, err)
		}
		body, _ := io.ReadAll(r.Body)
		r.Body.Close()
		if r.StatusCode != 200 || len(body) == 0 {
			t.Errorf("%s status=%d len=%d", path, r.StatusCode, len(body))
		}
		if cc := r.Header.Get("Cache-Control"); cc != "no-store, must-revalidate" {
			t.Errorf("%s Cache-Control = %q", path, cc)
		}
	}

	r, err := http.Get("http://" + addr + "/nope.js")
	if err != nil {
		t.Fatalf("GET /nope.js: %v", err)
	}
	r.Body.Close()
	if r.StatusCode != 404 {
		t.Errorf("/nope.js status = %d, want 404", r.StatusCode)
	}
}
