package webui

import (
	"io"
	"io/fs"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHandlerServesTheApp(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	rec := httptest.NewRecorder()
	Handler().ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("GET / = %d, want 200", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
		t.Errorf("Content-Type = %q, want text/html", ct)
	}
	body, _ := io.ReadAll(rec.Body)
	if !strings.Contains(string(body), `id="root"`) {
		t.Error("index.html is missing the app root")
	}
}

// TestBundledAssetsPresent guards against an empty or stale dist: the
// built app must ship exactly what index.html references.
func TestBundledAssetsPresent(t *testing.T) {
	sub, err := fs.Sub(dist, "dist")
	if err != nil {
		t.Fatal(err)
	}
	js, css := false, false
	if err := fs.WalkDir(sub, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		js = js || strings.HasSuffix(path, ".js")
		css = css || strings.HasSuffix(path, ".css")
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if !js || !css {
		t.Fatalf("dist is missing built assets (js: %v, css: %v); run npm run build in webui/", js, css)
	}
}
