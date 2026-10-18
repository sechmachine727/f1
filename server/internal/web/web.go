// Package web embeds the built dashboard. scripts/build_web.sh replaces dist/.
package web

import (
	"embed"
	"io/fs"
	"net/http"
	"path"
	"strings"
)

// all: includes dotfiles, so a fresh checkout with only .gitkeep still compiles.
//
//go:embed all:dist
var dist embed.FS

const notBuilt = `<!doctype html><html lang="en"><head><meta charset="utf-8"><title>F1 Telemetry</title></head>
<body style="font-family:system-ui;background:#0b0d11;color:#e5e7eb;padding:2rem">
<h1>Dashboard not built</h1>
<p>Run scripts/build_web.sh, then rebuild the server. The packaged Docker image and Windows ZIP already include the dashboard.</p>
</body></html>`

func FS() http.Handler {
	sub, err := fs.Sub(dist, "dist")
	if err != nil {
		return http.NotFoundHandler()
	}
	files := http.FileServer(http.FS(sub))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
		if p == "" {
			p = "index.html"
		}
		if _, err := fs.Stat(sub, p); err == nil {
			files.ServeHTTP(w, r)
			return
		}
		if strings.HasPrefix(p, "assets/") {
			http.NotFound(w, r)
			return
		}
		if _, err := fs.Stat(sub, "index.html"); err != nil {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(notBuilt))
			return
		}
		r.URL.Path = "/"
		files.ServeHTTP(w, r)
	})
}
