package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/josejuanmontiel/primaryfoods/internal/storage"
)

func TestSeasonalCalendarAPI(t *testing.T) {
	st, err := storage.NewSQLiteStorage(":memory:")
	if err != nil {
		t.Fatalf("failed to init storage: %v", err)
	}

	server := NewServer(st, nil, nil, nil, nil)

	// Test 1: Consultar mes 7 (julio - tomate, cereza, sardina, bonito)
	req := httptest.NewRequest("GET", "/api/seasonal/calendar?month=7", nil)
	w := httptest.NewRecorder()
	server.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	var res struct {
		Total int                      `json:"total"`
		Month int                      `json:"month"`
		Items []map[string]interface{} `json:"items"`
	}
	if err := json.NewDecoder(w.Body).Decode(&res); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if res.Month != 7 {
		t.Errorf("expected month 7, got %d", res.Month)
	}
	if res.Total == 0 {
		t.Errorf("expected seasonal items for month 7, got 0")
	}

	// Test 2: Filtrar por categoría 'fruit'
	req2 := httptest.NewRequest("GET", "/api/seasonal/calendar?month=7&category=fruit", nil)
	w2 := httptest.NewRecorder()
	server.mux.ServeHTTP(w2, req2)

	if w2.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w2.Code)
	}

	var res2 struct {
		Total int                      `json:"total"`
		Items []map[string]interface{} `json:"items"`
	}
	if err := json.NewDecoder(w2.Body).Decode(&res2); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	for _, it := range res2.Items {
		if it["category"] != "fruit" {
			t.Errorf("expected category 'fruit', got %v", it["category"])
		}
	}
}

func TestImportEndpoints(t *testing.T) {
	st, err := storage.NewSQLiteStorage(":memory:")
	if err != nil {
		t.Fatalf("failed to init storage: %v", err)
	}

	server := NewServer(st, nil, nil, nil, nil)

	// Test Import Seasonal
	seasonalPayload := `{"items": [{"primaryFoodId": "test_seasonal", "foodName": "Estacional Test", "category": "vegetable", "month": 5, "availability": "peak", "source": "TEST"}]}`
	req := httptest.NewRequest("POST", "/api/seasonal/import", strings.NewReader(seasonalPayload))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	server.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	// Test Import Cooking Profiles
	cookingPayload := `{"profiles": [{"primaryFoodId": "test_cooking", "foodName": "Coccion Test", "category": "cereal", "defaultEnergyClass": "minimal", "defaultFireMinutes": 2, "efficiencyTips": ["Test tip"]}]}`
	req2 := httptest.NewRequest("POST", "/api/cooking/import", strings.NewReader(cookingPayload))
	req2.Header.Set("Content-Type", "application/json")
	w2 := httptest.NewRecorder()
	server.mux.ServeHTTP(w2, req2)

	if w2.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w2.Code)
	}
}

