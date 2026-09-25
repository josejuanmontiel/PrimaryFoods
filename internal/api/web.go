package api

import (
	_ "embed"
	"net/http"
	"os"
)

//go:embed web/index.html
var defaultIndexHTML []byte

//go:embed web/openapi.yaml
var defaultOpenAPIYAML []byte

//go:embed web/openapi.json
var defaultOpenAPIJSON []byte

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

func (s *Server) handleOpenAPIYAML(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/yaml; charset=utf-8")
	if data, err := os.ReadFile("api/openapi.yaml"); err == nil {
		w.WriteHeader(http.StatusOK)
		w.Write(data)
		return
	}
	if data, err := os.ReadFile("internal/api/web/openapi.yaml"); err == nil {
		w.WriteHeader(http.StatusOK)
		w.Write(data)
		return
	}
	w.WriteHeader(http.StatusOK)
	w.Write(defaultOpenAPIYAML)
}

func (s *Server) handleOpenAPIJSON(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if data, err := os.ReadFile("api/openapi.json"); err == nil {
		w.WriteHeader(http.StatusOK)
		w.Write(data)
		return
	}
	if data, err := os.ReadFile("internal/api/web/openapi.json"); err == nil {
		w.WriteHeader(http.StatusOK)
		w.Write(data)
		return
	}
	w.WriteHeader(http.StatusOK)
	w.Write(defaultOpenAPIJSON)
}

func (s *Server) handleAPIDocs(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	html := `<!doctype html>
<html lang="es">
  <head>
    <title>PrimaryFoods API — Documentación Interactiva OpenAPI 3.1</title>
    <meta charset="utf-8" />
    <meta name="viewport" content="width=device-width, initial-scale=1" />
    <link rel="icon" href="data:image/svg+xml,<svg xmlns=%22http://www.w3.org/2000/svg%22 viewBox=%220 0 100 100%22><text y=%22.9em%22 font-size=%2290%22>🥦</text></svg>">
    <style>
      body { margin: 0; background: #0b0f19; font-family: system-ui, -apple-system, sans-serif; }
      .header-bar {
        background: #131b2e;
        color: #f1f5f9;
        padding: 0.75rem 1.5rem;
        display: flex;
        align-items: center;
        justify-content: space-between;
        border-bottom: 1px solid #233252;
      }
      .header-bar a {
        color: #10b981;
        text-decoration: none;
        font-weight: 500;
        font-size: 0.9rem;
      }
      .header-bar a:hover {
        text-decoration: underline;
      }
      .badge-api {
        background: rgba(16, 185, 129, 0.15);
        color: #34d399;
        padding: 0.25rem 0.6rem;
        border-radius: 6px;
        font-size: 0.8rem;
        border: 1px solid rgba(16, 185, 129, 0.3);
      }
    </style>
  </head>
  <body>
    <div class="header-bar">
      <div style="display: flex; align-items: center; gap: 0.75rem;">
        <span style="font-size: 1.4rem;">🥦</span>
        <strong>PrimaryFoods API v2.1.0</strong>
        <span class="badge-api">OpenAPI 3.1.0</span>
      </div>
      <div style="display: flex; gap: 1rem; align-items: center;">
        <a href="/">← Volver al Hub Web</a>
        <a href="/api/openapi.yaml" download="openapi.yaml">⬇️ YAML</a>
        <a href="/api/openapi.json" download="openapi.json">⬇️ JSON</a>
      </div>
    </div>
    <script
      id="api-reference"
      data-url="/api/openapi.yaml"
      data-configuration='{"theme": "deepSpace", "hideModels": false}'
      src="https://cdn.jsdelivr.net/npm/@scalar/api-reference"
    ></script>
  </body>
</html>`
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(html))
}
