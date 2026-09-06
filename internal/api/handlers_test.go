package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
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

func TestSyncBuscaturecetaAndMatchingEndpoint(t *testing.T) {
	srv := setupTestServer(t)

	// 1. Enviar paquete de Buscatureceta a POST /api/sync/buscatureceta
	pkg := model.BuscaturecetaPackage{
		SchemaVersion: "1.0",
		App:           "buscatureceta",
		ExportedAt:    "2026-09-03T20:00:00Z",
		Pantry: []model.PantryItem{
			{ID: 1, ProductCode: "84800001", ProductName: "Brócoli limpio fresco", Amount: 500, Unit: "g", PantryZone: "food"},
			{ID: 2, ProductCode: "84800002", ProductName: "Quinoa Bio", Amount: 400, Unit: "g", PantryZone: "food"},
		},
		Diary: []model.DiaryEntry{
			{
				ID:       1,
				Date:     "2026-09-03",
				MealType: "lunch",
				Status:   "consumed",
				Items: []model.DiaryItem{
					{Course: "main", Name: "Brócoli con Quinoa", Status: "consumed"},
				},
			},
		},
		DiaryVersions: []model.DiaryVersion{
			{
				ID:            1,
				DiaryEntryID:  1,
				Date:          "2026-09-03",
				MealType:      "lunch",
				Action:        "consumed",
				VersionNumber: 1,
				Status:        "consumed",
				Timestamp:     "2026-09-03T14:30:00Z",
			},
		},
	}

	pkgBytes, _ := json.Marshal(pkg)
	reqSync := httptest.NewRequest("POST", "/api/sync/buscatureceta", strings.NewReader(string(pkgBytes)))
	reqSync.Header.Set("Content-Type", "application/json")
	recSync := httptest.NewRecorder()

	srv.Handler().ServeHTTP(recSync, reqSync)
	if recSync.Code != http.StatusOK {
		t.Fatalf("Sync esperaba 200, obtenido %d: %s", recSync.Code, recSync.Body.String())
	}

	// 2. Verificar que GET /api/pantry devuelve los 2 items
	reqPantry := httptest.NewRequest("GET", "/api/pantry", nil)
	recPantry := httptest.NewRecorder()
	srv.Handler().ServeHTTP(recPantry, reqPantry)

	var pantryRes struct {
		Total int                `json:"total"`
		Items []model.PantryItem `json:"items"`
	}
	if err := json.Unmarshal(recPantry.Body.Bytes(), &pantryRes); err != nil {
		t.Fatalf("Error deserializando respuesta de despensa: %v", err)
	}
	if pantryRes.Total != 2 {
		t.Fatalf("Esperaba 2 productos en despensa, obtenidos %d", pantryRes.Total)
	}

	// 3. Evaluar Matching enviando recetas vía POST /api/recipes/matching
	reqMatchingBody := map[string]interface{}{
		"recipes": []model.Recipe{
			{
				ID:   "r-1",
				Slug: "brocoli-quinoa",
				Name: "Salteado de Brócoli y Quinoa",
				Ingredients: []model.RecipeIngredient{
					{Name: "Brócoli", Amount: 200, Unit: "g"},
					{Name: "Quinoa", Amount: 100, Unit: "g"},
					{Name: "Agua", Amount: 200, Unit: "ml"},
				},
			},
			{
				ID:   "r-2",
				Slug: "salmon-patatas",
				Name: "Salmón con Patatas",
				Ingredients: []model.RecipeIngredient{
					{Name: "Salmón", Amount: 200, Unit: "g"},
					{Name: "Patatas", Amount: 150, Unit: "g"},
				},
			},
		},
	}
	matchBytes, _ := json.Marshal(reqMatchingBody)
	reqMatch := httptest.NewRequest("POST", "/api/recipes/matching", strings.NewReader(string(matchBytes)))
	reqMatch.Header.Set("Content-Type", "application/json")
	recMatch := httptest.NewRecorder()

	srv.Handler().ServeHTTP(recMatch, reqMatch)
	if recMatch.Code != http.StatusOK {
		t.Fatalf("Matching esperaba 200, obtenido %d: %s", recMatch.Code, recMatch.Body.String())
	}

	var matchRes struct {
		TotalMatches     int                 `json:"totalMatches"`
		ReadyToCookCount int                 `json:"readyToCookCount"`
		Matches          []model.RecipeMatch `json:"matches"`
	}
	if err := json.Unmarshal(recMatch.Body.Bytes(), &matchRes); err != nil {
		t.Fatalf("Error deserializando matching: %v", err)
	}

	if matchRes.TotalMatches != 2 {
		t.Fatalf("Esperaba 2 recetas evaluadas, obtenidas %d", matchRes.TotalMatches)
	}
	if matchRes.ReadyToCookCount != 1 {
		t.Fatalf("Esperaba 1 receta 100%% lista para cocinar, obtenidas %d", matchRes.ReadyToCookCount)
	}
	if matchRes.Matches[0].Recipe.Slug != "brocoli-quinoa" || !matchRes.Matches[0].IsReadyToCook {
		t.Fatalf("La primera receta debe ser brocoli-quinoa lista para cocinar")
	}
}
