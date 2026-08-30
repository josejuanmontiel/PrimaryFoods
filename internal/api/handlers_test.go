package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/josejuanmontiel/primaryfoods/internal/model"
	"github.com/josejuanmontiel/primaryfoods/internal/storage"
)

func setupTestServer(t *testing.T) *Server {
	store, err := storage.NewSQLiteStorage(":memory:")
	if err != nil {
		t.Fatalf("error creando storage para test: %v", err)
	}
	return NewServer(store)
}

func TestHealthEndpoint(t *testing.T) {
	srv := setupTestServer(t)

	req := httptest.NewRequest("GET", "/api/health", nil)
	rec := httptest.NewRecorder()

	srv.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("Se esperaba status 200, obtenido %d", rec.Code)
	}

	var res map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &res); err != nil {
		t.Fatalf("Error deserializando JSON: %v", err)
	}
	if res["status"] != "ok" {
		t.Fatalf("Se esperaba status ok, obtenido %v", res["status"])
	}
}

func TestSearchEndpoint(t *testing.T) {
	srv := setupTestServer(t)

	req := httptest.NewRequest("GET", "/api/foods/search?q=brocoli", nil)
	rec := httptest.NewRecorder()

	srv.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("Se esperaba status 200, obtenido %d", rec.Code)
	}

	var res struct {
		Query   string              `json:"query"`
		Count   int                 `json:"count"`
		Results []model.PrimaryFood `json:"results"`
	}

	if err := json.Unmarshal(rec.Body.Bytes(), &res); err != nil {
		t.Fatalf("Error deserializando JSON: %v", err)
	}

	if res.Count == 0 || len(res.Results) == 0 {
		t.Fatalf("Se esperaba encontrar resultados para 'brocoli'")
	}
	if res.Results[0].ID != "brocoli" {
		t.Fatalf("El primer resultado debería ser 'brocoli', obtenido: %s", res.Results[0].ID)
	}
}

func TestExportEndpoint(t *testing.T) {
	srv := setupTestServer(t)

	req := httptest.NewRequest("GET", "/api/export/json", nil)
	rec := httptest.NewRecorder()

	srv.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("Se esperaba status 200, obtenido %d", rec.Code)
	}

	var foods []model.PrimaryFood
	if err := json.Unmarshal(rec.Body.Bytes(), &foods); err != nil {
		t.Fatalf("Error deserializando export JSON: %v", err)
	}

	if len(foods) == 0 {
		t.Fatalf("Se esperaba exportar alimentos, obtenido array vacío")
	}
}
