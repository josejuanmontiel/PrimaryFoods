package mealie

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/josejuanmontiel/primaryfoods/internal/model"
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

func TestClient_CreateOrUpdateRecipe(t *testing.T) {
	step := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-token" {
			t.Errorf("Esperaba token Bearer test-token, recibido %s", r.Header.Get("Authorization"))
		}

		if r.Method == "POST" && r.URL.Path == "/api/recipes" {
			step++
			w.WriteHeader(http.StatusCreated)
			w.Write([]byte(`"hummus-de-garbanzos"`))
			return
		}

		if r.Method == "GET" && r.URL.Path == "/api/recipes/hummus-de-garbanzos" {
			step++
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"id":"rec-456","slug":"hummus-de-garbanzos","name":"Hummus de Garbanzos"}`))
			return
		}

		if r.Method == "PATCH" && r.URL.Path == "/api/recipes/hummus-de-garbanzos" {
			step++
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"id":"rec-456","slug":"hummus-de-garbanzos"}`))
			return
		}

		t.Errorf("Ruta no esperada: %s %s", r.Method, r.URL.Path)
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-token")
	recipe := model.Recipe{
		Name:        "Hummus de Garbanzos",
		Description: "Hummus tradicional",
		Yield:       4,
		Ingredients: []model.RecipeIngredient{
			{Name: "garbanzos cocidos", Amount: 400, Unit: "g"},
		},
		Instructions: []string{"Triturar todo.", "Servir con pimentón."},
		Tags:         []string{"vegano", "legumbres"},
	}

	slug, err := client.CreateOrUpdateRecipe(context.Background(), recipe)
	if err != nil {
		t.Fatalf("CreateOrUpdateRecipe falló: %v", err)
	}
	if slug != "hummus-de-garbanzos" {
		t.Errorf("Slug incorrecto: %s", slug)
	}
	if step != 3 {
		t.Errorf("Esperaba 3 peticiones HTTP (POST, GET, PATCH), recibidas %d", step)
	}
}

func TestClient_CreateOrUpdateRecipe_WithImage(t *testing.T) {
	step := 0
	imageUploaded := false

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" && r.URL.Path == "/api/recipes" {
			step++
			w.WriteHeader(http.StatusCreated)
			w.Write([]byte(`"paella-valenciana"`))
			return
		}

		if r.Method == "GET" && r.URL.Path == "/api/recipes/paella-valenciana" {
			step++
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"id":"rec-789","slug":"paella-valenciana","name":"Paella Valenciana"}`))
			return
		}

		if r.Method == "PATCH" && r.URL.Path == "/api/recipes/paella-valenciana" {
			step++
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"id":"rec-789","slug":"paella-valenciana"}`))
			return
		}

		if r.Method == "PUT" && r.URL.Path == "/api/recipes/paella-valenciana/image" {
			step++
			imageUploaded = true
			if err := r.ParseMultipartForm(10 << 20); err != nil {
				t.Fatalf("Error parseando multipart form: %v", err)
			}
			ext := r.FormValue("extension")
			if ext != "jpg" && ext != "jpeg" {
				t.Errorf("Extensión inesperada: %s", ext)
			}
			file, _, err := r.FormFile("image")
			if err != nil {
				t.Fatalf("Campo image no encontrado en multipart: %v", err)
			}
			defer file.Close()
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"status":"success"}`))
			return
		}

		t.Errorf("Ruta no esperada: %s %s", r.Method, r.URL.Path)
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-token")
	recipe := model.Recipe{
		Name:        "Paella Valenciana",
		Description: "Auténtica paella",
		Yield:       4,
		Ingredients: []model.RecipeIngredient{
			{Name: "arroz", Amount: 400, Unit: "g"},
		},
		Instructions: []string{"Sofreír.", "Añadir caldo.", "Cocer arroz."},
		Image:        "data:image/jpeg;base64,/9j/4AAQSkZJRgABAQEASABIAAD/2wBDAP//////////////////////////////////////////////////////////////////////////////////////wgALCAABAAEBAREA/8QAFBABAAAAAAAAAAAAAAAAAAAAAP/aAAgBAQABPxA=",
	}

	slug, err := client.CreateOrUpdateRecipe(context.Background(), recipe)
	if err != nil {
		t.Fatalf("CreateOrUpdateRecipe con imagen falló: %v", err)
	}
	if slug != "paella-valenciana" {
		t.Errorf("Slug incorrecto: %s", slug)
	}
	if !imageUploaded {
		t.Errorf("La imagen no fue subida a Mealie")
	}
	if step != 4 {
		t.Errorf("Esperaba 4 peticiones HTTP (POST, GET, PATCH, PUT /image), recibidas %d", step)
	}
}
