package storage

import (
	"testing"

	"github.com/josejuanmontiel/primaryfoods/internal/model"
)

func setupTestStorage(t *testing.T) *Storage {
	s, err := NewSQLiteStorage(":memory:")
	if err != nil {
		t.Fatalf("error creando storage en memoria: %v", err)
	}
	return s
}

func TestSeedAndSearch(t *testing.T) {
	s := setupTestStorage(t)
	defer s.Close()

	// 1. Debe haber precargado los alimentos semilla
	foods, total, err := s.GetAll(model.SearchFilter{})
	if err != nil {
		t.Fatalf("GetAll falló: %v", err)
	}
	if total == 0 || len(foods) == 0 {
		t.Fatalf("Se esperaba alimentos semilla precargados, total: %d", total)
	}

	// 2. Búsqueda exacta por nombre
	results, err := s.Search("Brócoli", 5)
	if err != nil {
		t.Fatalf("Search falló: %v", err)
	}
	if len(results) == 0 || results[0].ID != "brocoli" {
		t.Fatalf("Se esperaba encontrar brócoli, obtenido: %+v", results)
	}

	// 3. Búsqueda por alias / sinónimo en minúsculas sin tilde
	results, err = s.Search("brocolis", 5)
	if err != nil {
		t.Fatalf("Search por alias falló: %v", err)
	}
	if len(results) == 0 || results[0].ID != "brocoli" {
		t.Fatalf("Se esperaba encontrar brócoli por alias 'brocolis', obtenido: %+v", results)
	}

	// 4. Búsqueda de avena
	results, err = s.Search("copos de avena", 5)
	if err != nil {
		t.Fatalf("Search avena falló: %v", err)
	}
	if len(results) == 0 || results[0].ID != "avena_hojuelas" {
		t.Fatalf("Se esperaba encontrar avena por alias, obtenido: %+v", results)
	}
}

func TestUpsertAndGetByID(t *testing.T) {
	s := setupTestStorage(t)
	defer s.Close()

	newFood := model.PrimaryFood{
		ID:       "moringa",
		Name:     "Moringa en polvo",
		Category: "superfood",
		Aliases:  []string{"moringa", "hojas de moringa"},
		NutritionPer100g: model.Nutrition{
			Calories: 205.0,
			Protein:  27.1,
		},
		Benefits: []string{"Gran densidad de micronutrientes"},
		Tags:     []string{"antioxidante", "superalimento"},
	}

	if err := s.Upsert(newFood); err != nil {
		t.Fatalf("Upsert falló: %v", err)
	}

	fetched, err := s.GetByID("moringa")
	if err != nil {
		t.Fatalf("GetByID falló: %v", err)
	}
	if fetched == nil || fetched.Name != "Moringa en polvo" {
		t.Fatalf("Se esperaba obtener moringa, obtenido: %+v", fetched)
	}
}
