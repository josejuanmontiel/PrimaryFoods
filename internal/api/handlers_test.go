package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/josejuanmontiel/primaryfoods/internal/mealie"
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

func TestRecipeImportAndAnalyzeEndpoint(t *testing.T) {
	srv := setupTestServer(t)

	// Guardar un producto comercial de prueba en el storage
	_ = srv.storage.SaveCommercialProducts([]model.CommercialProduct{
		{
			Code:            "8410001",
			ProductName:     "Macarrones Gallo",
			Brands:          "Gallo",
			NutriscoreGrade: "a",
			NovaGroup:       "1",
		},
		{
			Code:            "8410002",
			ProductName:     "Masa Quebrada Buitoni",
			Brands:          "Buitoni",
			NutriscoreGrade: "c",
			NovaGroup:       "3",
		},
	})

	// 1. Importar Receta JSON
	recipeJSON := `[
		{
			"name": "Macarrones Rápidos",
			"servings": 2,
			"ingredients": [
				{"name": "Macarrones", "amount": 200, "unit": "g"},
				{"name": "Brócoli", "amount": 100, "unit": "g"},
				{"name": "Masa Quebrada", "amount": 1, "unit": "unidad"}
			]
		}
	]`
	reqImport := httptest.NewRequest("POST", "/api/recipes/import-json", strings.NewReader(recipeJSON))
	recImport := httptest.NewRecorder()
	srv.Handler().ServeHTTP(recImport, reqImport)

	if recImport.Code != http.StatusOK {
		t.Fatalf("Esperaba status 200 en importación, obtenido %d: %s", recImport.Code, recImport.Body.String())
	}

	// 2. Analizar Receta
	reqAnalyze := httptest.NewRequest("GET", "/api/recipes/analyze?slug=macarrones-rapidos", nil)
	recAnalyze := httptest.NewRecorder()
	srv.Handler().ServeHTTP(recAnalyze, reqAnalyze)

	if recAnalyze.Code != http.StatusOK {
		t.Fatalf("Esperaba status 200 en análisis, obtenido %d: %s", recAnalyze.Code, recAnalyze.Body.String())
	}

	var res struct {
		TotalRecipes          int                    `json:"totalRecipes"`
		TotalIngredients      int                    `json:"totalIngredients"`
		PrimaryMatches        int                    `json:"primaryMatches"`
		CommercialMatches     int                    `json:"commercialMatches"`
		GlobalMatchPercentage float64                `json:"globalMatchPercentage"`
		Analyses              []model.RecipeAnalysis `json:"analyses"`
	}
	if err := json.Unmarshal(recAnalyze.Body.Bytes(), &res); err != nil {
		t.Fatalf("Error parseando resultado de análisis: %v", err)
	}

	if res.TotalIngredients != 3 {
		t.Fatalf("Esperaba 3 ingredientes, obtenidos %d", res.TotalIngredients)
	}
	// Brócoli es alimento primario
	if res.PrimaryMatches < 1 {
		t.Fatalf("Esperaba al menos 1 match primario (Brócoli), obtenidos %d", res.PrimaryMatches)
	}
	// Macarrones o Masa Quebrada es comercial
	if res.CommercialMatches < 1 {
		t.Fatalf("Esperaba al menos 1 match comercial (Macarrones/Masa Quebrada), obtenidos %d", res.CommercialMatches)
	}
	if res.GlobalMatchPercentage < 66.0 {
		t.Fatalf("Esperaba porcentaje de match > 66%%, obtenido %.1f%%", res.GlobalMatchPercentage)
	}
}

func TestOpenAPIAndBuscaturecetaExport(t *testing.T) {
	srv := setupTestServer(t)

	// 1. Probar OpenAPI YAML
	reqYaml := httptest.NewRequest("GET", "/api/openapi.yaml", nil)
	recYaml := httptest.NewRecorder()
	srv.Handler().ServeHTTP(recYaml, reqYaml)

	if recYaml.Code != http.StatusOK {
		t.Fatalf("GET /api/openapi.yaml esperaba 200, obtenido %d", recYaml.Code)
	}
	if !strings.Contains(recYaml.Body.String(), "openapi: 3.1.0") {
		t.Fatalf("El YAML no contiene 'openapi: 3.1.0'")
	}

	// 2. Probar OpenAPI JSON
	reqJSON := httptest.NewRequest("GET", "/api/openapi.json", nil)
	recJSON := httptest.NewRecorder()
	srv.Handler().ServeHTTP(recJSON, reqJSON)

	if recJSON.Code != http.StatusOK {
		t.Fatalf("GET /api/openapi.json esperaba 200, obtenido %d", recJSON.Code)
	}
	var openapiObj map[string]interface{}
	if err := json.Unmarshal(recJSON.Body.Bytes(), &openapiObj); err != nil {
		t.Fatalf("Error deserializando OpenAPI JSON: %v", err)
	}
	if openapiObj["openapi"] != "3.1.0" {
		t.Fatalf("Esperaba versión openapi 3.1.0, obtenida %v", openapiObj["openapi"])
	}

	// 3. Probar /docs
	reqDocs := httptest.NewRequest("GET", "/docs", nil)
	recDocs := httptest.NewRecorder()
	srv.Handler().ServeHTTP(recDocs, reqDocs)
	if recDocs.Code != http.StatusOK {
		t.Fatalf("GET /docs esperaba 200, obtenido %d", recDocs.Code)
	}
	if !strings.Contains(recDocs.Body.String(), "api-reference") {
		t.Fatalf("/docs no contiene api-reference de Scalar")
	}

	// 4. Probar exportación Buscatureceta
	reqExport := httptest.NewRequest("GET", "/api/export/buscatureceta", nil)
	recExport := httptest.NewRecorder()
	srv.Handler().ServeHTTP(recExport, reqExport)

	if recExport.Code != http.StatusOK {
		t.Fatalf("GET /api/export/buscatureceta esperaba 200, obtenido %d: %s", recExport.Code, recExport.Body.String())
	}
	var exportPack BuscaturecetaEnrichedExport
	if err := json.Unmarshal(recExport.Body.Bytes(), &exportPack); err != nil {
		t.Fatalf("Error deserializando exportación de Buscatureceta: %v", err)
	}
	if exportPack.App != "buscatureceta" {
		t.Fatalf("Esperaba app='buscatureceta', obtenida '%s'", exportPack.App)
	}

	// 5. Probar formato recipes_only
	reqRecOnly := httptest.NewRequest("GET", "/api/export/buscatureceta?format=recipes_only", nil)
	recRecOnly := httptest.NewRecorder()
	srv.Handler().ServeHTTP(recRecOnly, reqRecOnly)
	if recRecOnly.Code != http.StatusOK {
		t.Fatalf("GET /api/export/buscatureceta?format=recipes_only esperaba 200, obtenido %d", recRecOnly.Code)
	}
	var recipesOnly []BuscaturecetaEnrichedRecipe
	if err := json.Unmarshal(recRecOnly.Body.Bytes(), &recipesOnly); err != nil {
		t.Fatalf("Error deserializando recipes_only: %v", err)
	}
}

func TestMealieExportEndpoint(t *testing.T) {
	srv := setupTestServer(t)

	// Mock server de Mealie
	mealieServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" && r.URL.Path == "/api/recipes" {
			w.WriteHeader(http.StatusCreated)
			w.Write([]byte(`"lentejas-estofadas"`))
			return
		}
		if r.Method == "GET" && r.URL.Path == "/api/recipes/lentejas-estofadas" {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"slug":"lentejas-estofadas","name":"Lentejas Estofadas"}`))
			return
		}
		if r.Method == "PATCH" && r.URL.Path == "/api/recipes/lentejas-estofadas" {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"slug":"lentejas-estofadas"}`))
			return
		}
		http.NotFound(w, r)
	}))
	defer mealieServer.Close()

	srv.mealieClient = mealie.NewClient(mealieServer.URL, "test-token")

	// Guardar una receta en el storage
	_ = srv.storage.SaveRecipes([]model.Recipe{
		{
			ID:          "rec-lent",
			Slug:        "lentejas-estofadas",
			Name:        "Lentejas Estofadas",
			Description: "Ricas lentejas",
			Yield:       4,
			Ingredients: []model.RecipeIngredient{
				{Name: "Lentejas", Amount: 300, Unit: "g"},
				{Name: "Zanahoria", Amount: 2, Unit: "unidad"},
			},
			Instructions: []string{"Cocer las lentejas con verduras."},
			Tags:         []string{"tradicional", "legumbres"},
		},
	})

	// 1. Exportar vía POST /api/mealie/export?slug=lentejas-estofadas
	reqExport := httptest.NewRequest("POST", "/api/mealie/export?slug=lentejas-estofadas", nil)
	recExport := httptest.NewRecorder()
	srv.Handler().ServeHTTP(recExport, reqExport)

	if recExport.Code != http.StatusOK {
		t.Fatalf("POST /api/mealie/export esperaba 200, obtenido %d: %s", recExport.Code, recExport.Body.String())
	}

	var res struct {
		Status        string `json:"status"`
		TotalTarget   int    `json:"totalTarget"`
		TotalExported int    `json:"totalExported"`
		TotalFailed   int    `json:"totalFailed"`
		Exported      []struct {
			Name string `json:"name"`
			Slug string `json:"slug"`
			URL  string `json:"url"`
		} `json:"exported"`
	}
	if err := json.Unmarshal(recExport.Body.Bytes(), &res); err != nil {
		t.Fatalf("Error deserializando respuesta: %v", err)
	}

	if res.Status != "ok" || res.TotalExported != 1 {
		t.Fatalf("Esperaba 1 exportada exitosa, obtenidos: %+v", res)
	}
	if res.Exported[0].Slug != "lentejas-estofadas" {
		t.Fatalf("Slug inesperado: %s", res.Exported[0].Slug)
	}
}

func TestConfigEndpoints(t *testing.T) {
	srv := setupTestServer(t)

	// 1. GET /api/config
	reqGet := httptest.NewRequest("GET", "/api/config", nil)
	recGet := httptest.NewRecorder()
	srv.Handler().ServeHTTP(recGet, reqGet)
	if recGet.Code != http.StatusOK {
		t.Fatalf("GET /api/config esperaba 200, obtenido %d", recGet.Code)
	}

	// 2. POST /api/config
	updatePayload := `{"mealieUrl":"http://localhost:9930","mealieToken":"new-secret-token"}`
	reqPost := httptest.NewRequest("POST", "/api/config", strings.NewReader(updatePayload))
	reqPost.Header.Set("Content-Type", "application/json")
	recPost := httptest.NewRecorder()
	srv.Handler().ServeHTTP(recPost, reqPost)
	if recPost.Code != http.StatusOK {
		t.Fatalf("POST /api/config esperaba 200, obtenido %d: %s", recPost.Code, recPost.Body.String())
	}

	if srv.mealieClient.BaseURL != "http://localhost:9930" {
		t.Errorf("BaseURL esperado http://localhost:9930, obtenido %s", srv.mealieClient.BaseURL)
	}
}

func TestMealieDiffAndImport(t *testing.T) {
	srv := setupTestServer(t)

	// 1. Guardar receta local en storage
	_ = srv.storage.SaveRecipes([]model.Recipe{
		{
			ID:          "rec-lent",
			Slug:        "lentejas-estofadas",
			Name:        "Lentejas Estofadas",
			Description: "Lentejas de la abuela",
			Yield:       4,
			Ingredients: []model.RecipeIngredient{
				{Name: "Lentejas", Amount: 300, Unit: "g"},
			},
		},
	})

	// 2. Mock de Mealie con 2 recetas (una existente en sqlite y una nueva de redes sociales)
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" && r.URL.Path == "/api/recipes" {
			res := map[string]interface{}{
				"page":    1,
				"perPage": 10,
				"total":   2,
				"items": []map[string]interface{}{
					{
						"id":          "rec-carb-1",
						"slug":        "garbanzos-carbonara",
						"name":        "Garbanzos Carbonara Saludable",
						"recipeYield": 2,
						"recipeIngredient": []map[string]interface{}{
							{"display": "400g garbanzos cocidos"},
							{"display": "2 huevos"},
						},
						"recipeInstructions": []map[string]interface{}{
							{"text": "Saltear los garbanzos y añadir huevo batido."},
						},
					},
					{
						"id":          "rec-lent",
						"slug":        "lentejas-estofadas",
						"name":        "Lentejas Estofadas",
						"recipeYield": 4,
						"recipeIngredient": []map[string]interface{}{
							{"display": "300g lentejas"},
						},
					},
				},
			}
			json.NewEncoder(w).Encode(res)
			return
		}

		if r.Method == "GET" && r.URL.Path == "/api/recipes/garbanzos-carbonara" {
			detail := map[string]interface{}{
				"id":          "rec-carb-1",
				"slug":        "garbanzos-carbonara",
				"name":        "Garbanzos Carbonara Saludable",
				"recipeYield": 2,
				"recipeIngredient": []map[string]interface{}{
					{"display": "400g garbanzos cocidos"},
					{"display": "2 huevos"},
				},
				"recipeInstructions": []map[string]interface{}{
					{"text": "Saltear los garbanzos y añadir huevo batido."},
				},
			}
			json.NewEncoder(w).Encode(detail)
			return
		}

		http.NotFound(w, r)
	}))
	defer mockServer.Close()

	srv.mealieClient = mealie.NewClient(mockServer.URL, "token")

	// 3. Probar GET /api/mealie/diff
	reqDiff := httptest.NewRequest("GET", "/api/mealie/diff", nil)
	recDiff := httptest.NewRecorder()
	srv.Handler().ServeHTTP(recDiff, reqDiff)
	if recDiff.Code != http.StatusOK {
		t.Fatalf("GET /api/mealie/diff esperaba 200, obtenido %d: %s", recDiff.Code, recDiff.Body.String())
	}

	var diffRes MealieDiffResult
	if err := json.Unmarshal(recDiff.Body.Bytes(), &diffRes); err != nil {
		t.Fatalf("Error deserializando diff: %v", err)
	}

	if diffRes.TotalMealie != 2 {
		t.Errorf("TotalMealie esperado 2, obtenido %d", diffRes.TotalMealie)
	}
	if diffRes.NewInMealieCount != 1 {
		t.Errorf("NewInMealieCount esperado 1, obtenido %d", diffRes.NewInMealieCount)
	}
	if diffRes.InBothCount != 1 {
		t.Errorf("InBothCount esperado 1, obtenido %d", diffRes.InBothCount)
	}
	if len(diffRes.Items) < 1 || diffRes.Items[0].Slug != "garbanzos-carbonara" || diffRes.Items[0].Status != "new_in_mealie" {
		t.Errorf("Primer item esperado garbanzos-carbonara como new_in_mealie: %+v", diffRes.Items)
	}

	// 4. Probar POST /api/mealie/import?slug=garbanzos-carbonara
	reqImp := httptest.NewRequest("POST", "/api/mealie/import?slug=garbanzos-carbonara", nil)
	recImp := httptest.NewRecorder()
	srv.Handler().ServeHTTP(recImp, reqImp)
	if recImp.Code != http.StatusOK {
		t.Fatalf("POST /api/mealie/import esperaba 200, obtenido %d: %s", recImp.Code, recImp.Body.String())
	}

	var impRes struct {
		Status        string `json:"status"`
		TotalImported int    `json:"totalImported"`
	}
	if err := json.Unmarshal(recImp.Body.Bytes(), &impRes); err != nil {
		t.Fatalf("Error deserializando import: %v", err)
	}
	if impRes.Status != "ok" || impRes.TotalImported != 1 {
		t.Fatalf("Resultado de importación no esperado: %+v", impRes)
	}

	// 5. Verificar que ahora la receta está en storage
	all, err := srv.storage.GetAllRecipes()
	if err != nil {
		t.Fatalf("Error consultando storage: %v", err)
	}
	if len(all) != 2 {
		t.Fatalf("Se esperaban 2 recetas en storage, encontradas %d", len(all))
	}
}


