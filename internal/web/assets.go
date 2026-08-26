package web

import (
	"embed"
	"net/http"
)

//go:embed frontend/index.html frontend/app.css frontend/app.js
var frontend embed.FS

func serveAsset(w http.ResponseWriter, name, contentType string) {
	b, err := frontend.ReadFile("frontend/" + name)
	if err != nil {
		http.Error(w, "资源不存在", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Cache-Control", "no-cache")
	_, _ = w.Write(b)
}

func (h *Handler) WorkbenchHandler(w http.ResponseWriter, r *http.Request) {
	serveAsset(w, "index.html", "text/html; charset=utf-8")
}
func (h *Handler) StylesHandler(w http.ResponseWriter, r *http.Request) {
	serveAsset(w, "app.css", "text/css; charset=utf-8")
}
func (h *Handler) ScriptHandler(w http.ResponseWriter, r *http.Request) {
	serveAsset(w, "app.js", "text/javascript; charset=utf-8")
}
