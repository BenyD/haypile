// Package webui is the bundled single-user web interface, served by the
// daemon at /. The source lives in webui/ at the repo root (Vite +
// Preact + Tailwind); `npm run build` there writes dist/, which is
// committed so a plain `go build` always carries the current UI. CI
// rebuilds it and fails on drift. Free and AGPL like the rest of the
// single-user product.
package webui

import (
	"embed"
	"io/fs"
	"net/http"
	"strings"
)

//go:embed all:dist
var dist embed.FS

// Handler serves the web UI. index.html answers at /.
//
// Caching is explicit because embedded files carry no modification time,
// which otherwise invites browsers to heuristically cache the app shell
// and keep running a stale UI after the binary updates. Hashed assets
// are immutable by construction; everything else must revalidate.
func Handler() http.Handler {
	sub, err := fs.Sub(dist, "dist")
	if err != nil {
		panic(err) // embedded tree is fixed at compile time
	}
	files := http.FileServerFS(sub)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/assets/") {
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		} else {
			w.Header().Set("Cache-Control", "no-cache")
		}
		files.ServeHTTP(w, r)
	})
}
