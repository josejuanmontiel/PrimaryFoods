package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/josejuanmontiel/primaryfoods/internal/model"
	"github.com/josejuanmontiel/primaryfoods/internal/storage"
)

func TestMenuGenerationAPI(t *testing.T) {
	st, err := storage.NewSQLiteStorage(":memory:")
	if err != nil {
		t.Fatalf("failed to init storage: %v", err)
	}

	// Guardar unas recetas de prueba en la BD
	recipes := []model.Recipe{
		{
			ID:   "rec-1",
			Slug: "lentejas-estofadas",
			Name: "Lentejas Estofadas",
			Ingredients: []model.RecipeIngredient{
				{Name: "Lentejas"}, {Name: "Zanahoria"},
			},
		},
		{
			ID:   "rec-2",
			Slug: "salmon-plancha",
			Name: "Salmón a la Plancha",
			Ingredients: []model.RecipeIngredient{
				{Name: "Salmón"}, {Name: "Espinacas"},
			},
		},
	}
	if err := st.SaveRecipes(recipes); err != nil {
		t.Fatalf("error saving recipes: %v", err)
	}

	server := NewServer(st, nil)

	// Test GET /api/menu/generate?week=28&year=2026
	req := httptest.NewRequest("GET", "/api/menu/generate?week=28&year=2026&prioritize_pantry=true", nil)
	w := httptest.NewRecorder()
	server.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var plan model.WeeklyMenuPlan
	if err := json.NewDecoder(w.Body).Decode(&plan); err != nil {
		t.Fatalf("failed to decode menu plan: %v", err)
	}

	if plan.TotalMeals != 14 {
		t.Errorf("expected 14 total meals, got %d", plan.TotalMeals)
	}

	if plan.WeekNumber != 28 {
		t.Errorf("expected week 28, got %d", plan.WeekNumber)
	}

	if len(plan.Slots) != 14 {
		t.Errorf("expected 14 slots, got %d", len(plan.Slots))
	}

	// Test 2: GET /api/menu/generate con fuel_saving=true
	reqFuel := httptest.NewRequest("GET", "/api/menu/generate?week=28&year=2026&fuel_saving=true", nil)
	wFuel := httptest.NewRecorder()
	server.mux.ServeHTTP(wFuel, reqFuel)
	if wFuel.Code != http.StatusOK {
		t.Fatalf("expected status 200 with fuel_saving, got %d", wFuel.Code)
	}

	// Test 3: GET /api/cooking/energy-profiles
	reqProfiles := httptest.NewRequest("GET", "/api/cooking/energy-profiles", nil)
	wProfiles := httptest.NewRecorder()
	server.mux.ServeHTTP(wProfiles, reqProfiles)
	if wProfiles.Code != http.StatusOK {
		t.Fatalf("expected status 200 for cooking profiles, got %d", wProfiles.Code)
	}
	var resProfiles struct {
		Total    int                        `json:"total"`
		Profiles []model.FoodCookingProfile `json:"profiles"`
	}
	if err := json.NewDecoder(wProfiles.Body).Decode(&resProfiles); err != nil {
		t.Fatalf("failed to decode cooking profiles response: %v", err)
	}
	if resProfiles.Total == 0 {
		t.Errorf("expected cooking profiles, got 0")
	}
}

func TestAnnualMenuGenerationAPI(t *testing.T) {
	st, err := storage.NewSQLiteStorage(":memory:")
	if err != nil {
		t.Fatalf("failed to init storage: %v", err)
	}

	recipes := []model.Recipe{
		{
			ID:   "rec-1",
			Slug: "lentejas-estofadas",
			Name: "Lentejas Estofadas",
			Ingredients: []model.RecipeIngredient{
				{Name: "Lentejas"}, {Name: "Zanahoria"},
			},
		},
		{
			ID:   "rec-2",
			Slug: "salmon-plancha",
			Name: "Salmón a la Plancha",
			Ingredients: []model.RecipeIngredient{
				{Name: "Salmón"}, {Name: "Espinacas"},
			},
		},
		{
			ID:   "rec-3",
			Slug: "ensalada-tomate",
			Name: "Ensalada de Tomate",
			Ingredients: []model.RecipeIngredient{
				{Name: "Tomate"}, {Name: "Aceite"},
			},
		},
	}
	if err := st.SaveRecipes(recipes); err != nil {
		t.Fatalf("error saving recipes: %v", err)
	}

	server := NewServer(st, nil)

	// 1. GET /api/menu/annual
	req := httptest.NewRequest("GET", "/api/menu/annual?year=2027&min_days=5", nil)
	w := httptest.NewRecorder()
	server.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var annualPlan model.AnnualMenuPlan
	if err := json.NewDecoder(w.Body).Decode(&annualPlan); err != nil {
		t.Fatalf("failed to decode annual plan: %v", err)
	}

	if annualPlan.TotalWeeks != 52 {
		t.Errorf("expected 52 weeks, got %d", annualPlan.TotalWeeks)
	}
	if annualPlan.TotalMeals != 52*14 {
		t.Errorf("expected %d meals, got %d", 52*14, annualPlan.TotalMeals)
	}

	// 2. GET /api/menu/annual/export (Buscatureceta Package format)
	reqExport := httptest.NewRequest("GET", "/api/menu/annual/export?year=2027", nil)
	wExport := httptest.NewRecorder()
	server.mux.ServeHTTP(wExport, reqExport)

	if wExport.Code != http.StatusOK {
		t.Fatalf("expected status 200 for export, got %d", wExport.Code)
	}

	var pkg model.BuscaturecetaPackage
	if err := json.NewDecoder(wExport.Body).Decode(&pkg); err != nil {
		t.Fatalf("failed to decode buscatureceta package: %v", err)
	}

	if len(pkg.Diary) != 52*14 {
		t.Errorf("expected %d diary entries, got %d", 52*14, len(pkg.Diary))
	}
}

