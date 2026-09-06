package api

import (
	_ "embed"
	"net/http"
	"os"
)

//go:embed web/index.html
var defaultIndexHTML []byte

func (s *Server) handleIndexUI(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" && r.URL.Path != "/index.html" {
		http.NotFound(w, r)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	// Si existe en disco (durante desarrollo local), servir desde disco para recarga en caliente
	if data, err := os.ReadFile("web/index.html"); err == nil {
		w.WriteHeader(http.StatusOK)
		w.Write(data)
		return
	}
	if data, err := os.ReadFile("internal/api/web/index.html"); err == nil {
		w.WriteHeader(http.StatusOK)
		w.Write(data)
		return
	}

	// Si no, servir el binario embebido
	w.WriteHeader(http.StatusOK)
	w.Write(defaultIndexHTML)
}
