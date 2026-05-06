package configui

import (
	"embed"
	"io/fs"
	"net/http"
	"os"
)

//go:embed dist
var distFS embed.FS

// staticHandler returns an http.Handler that serves the embedded frontend,
// falling back to index.html for client-side routing (SPA mode).
// In dev mode (INSIDER_DEV=1), returns nil so the server skips static files.
func staticHandler() http.Handler {
	if os.Getenv("INSIDER_DEV") != "" {
		return nil
	}
	sub, err := fs.Sub(distFS, "dist")
	if err != nil {
		return nil
	}
	return &spaHandler{fs: http.FS(sub)}
}

// spaHandler serves static files and falls back to index.html for unknown paths.
type spaHandler struct {
	fs http.FileSystem
}

func (h *spaHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f, err := h.fs.Open(r.URL.Path)
	if err != nil {
		// Fallback to index.html for SPA routing
		r2 := *r
		r2.URL.Path = "/"
		http.FileServer(h.fs).ServeHTTP(w, &r2)
		return
	}
	f.Close()
	http.FileServer(h.fs).ServeHTTP(w, r)
}
