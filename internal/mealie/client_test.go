package mealie

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestParseMealieIngredient(t *testing.T) {
	// Caso 1: Estructurado con Food y Unit
	raw1 := MealieIngredientRaw{
		Food:     map[string]interface{}{"name": "Brócoli"},
		Quantity: 250.0,
		Unit:     map[string]interface{}{"name": "g"},
		Display:  "250g Brócoli",
	}
	ing1 := ParseMealieIngredient(raw1)
	if ing1.Name != "Brócoli" || ing1.Amount != 250 || ing1.Unit != "g" {
		t.Errorf("Error parseando ingrediente estructurado: %+v", ing1)
	}

	// Caso 2: Texto libre con unidades
	raw2 := MealieIngredientRaw{
		Display: "150ml aceite de oliva virgen extra",
	}
	ing2 := ParseMealieIngredient(raw2)
	if ing2.Amount != 150 || ing2.Unit != "ml" || ing2.Name != "aceite de oliva virgen extra" {
		t.Errorf("Error parseando ingrediente con display: %+v", ing2)
	}

	// Caso 3: Fracción
	raw3 := MealieIngredientRaw{
		Display: "1/2 cebolla picada",
	}
	ing3 := ParseMealieIngredient(raw3)
	if ing3.Amount != 0.5 || ing3.Name != "cebolla picada" {
		t.Errorf("Error parseando fracción: %+v", ing3)
	}
}

func TestClient_GetRecipes(t *testing.T) {
	mockResponse := MealieListResponse{
		Page:    1,
		PerPage: 10,
		Total:   1,
		Items: []MealieRecipeDetail{
			{
				ID:          "rec-123",
				Slug:        "guacamole-casero",
				Name:        "Guacamole Casero",
				Description: "Clásico guacamole mexicano",
				RecipeYield: 4,
				RecipeIngredient: []MealieIngredientRaw{
					{
						Food:     map[string]interface{}{"name": "Aguacate"},
						Quantity: 3.0,
						Unit:     map[string]interface{}{"name": "unidad"},
					},
					{
						Display: "1 tomate maduro en dados",
					},
				},
				RecipeInstructions: []interface{}{
					map[string]interface{}{"text": "Machaca los aguacates."},
					map[string]interface{}{"text": "Mezcla con el tomate."},
				},
				Tags: []interface{}{"mexicano", "aperitivo"},
			},
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-token" {
			t.Errorf("Esperaba token Bearer test-token, recibido %s", r.Header.Get("Authorization"))
		}
		json.NewEncoder(w).Encode(mockResponse)
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-token")
	recipes, err := client.GetRecipes(context.Background())
	if err != nil {
		t.Fatalf("GetRecipes falló: %v", err)
	}

	if len(recipes) != 1 {
		t.Fatalf("Esperaba 1 receta, recibidas %d", len(recipes))
	}

	r := recipes[0]
	if r.Name != "Guacamole Casero" {
		t.Errorf("Nombre incorrecto: %s", r.Name)
	}
	if len(r.Ingredients) != 2 {
		t.Errorf("Esperaba 2 ingredientes, recibidos %d", len(r.Ingredients))
	}
	if r.Ingredients[0].Name != "Aguacate" {
		t.Errorf("Ingrediente 1 incorrecto: %s", r.Ingredients[0].Name)
	}
}
