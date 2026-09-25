package importer

import (
	"strings"
	"testing"
)

func TestLoadRecipesFromJSON_SchemaOrgWithImage(t *testing.T) {
	jsonPayload := `{
		"@context": "https://schema.org",
		"@type": "Recipe",
		"name": "Spaghetti Siciliana",
		"description": "Pasta con anchoas y aceitunas",
		"image": "data:image/jpeg;base64,/9j/4AAQSkZJRg==",
		"recipeYield": "4 raciones",
		"recipeIngredient": [
			"500 g de spaghetti",
			"10 unidades de aceitunas negras"
		],
		"recipeInstructions": [
			{
				"@type": "HowToStep",
				"text": "Cocer la pasta al dente."
			},
			{
				"@type": "HowToStep",
				"text": "Mezclar con la salsa."
			}
		],
		"ingredients": [
			{"name": "spaghetti", "amount": 500, "unit": "g"},
			{"name": "aceitunas negras", "amount": 10, "unit": "unidades"}
		],
		"instructions": [
			"Cocer la pasta al dente.",
			"Mezclar con la salsa."
		],
		"servings": 4
	}`

	recipes, err := LoadRecipesFromJSON(strings.NewReader(jsonPayload))
	if err != nil {
		t.Fatalf("LoadRecipesFromJSON falló: %v", err)
	}

	if len(recipes) != 1 {
		t.Fatalf("Esperaba 1 receta, obtenidas %d", len(recipes))
	}

	r := recipes[0]
	if r.Name != "Spaghetti Siciliana" {
		t.Errorf("Nombre incorrecto: %s", r.Name)
	}
	if r.Image != "data:image/jpeg;base64,/9j/4AAQSkZJRg==" {
		t.Errorf("Imagen incorrecta: %s", r.Image)
	}
	if r.Yield != 4 {
		t.Errorf("Raciones incorrectas: %d", r.Yield)
	}
	if len(r.Ingredients) != 2 {
		t.Errorf("Ingredientes esperados 2, obtenidos %d", len(r.Ingredients))
	}
	if len(r.Instructions) != 2 {
		t.Errorf("Instrucciones esperadas 2, obtenidas %d", len(r.Instructions))
	}
}

func TestLoadRecipesFromJSON_PureSchemaOrg(t *testing.T) {
	jsonPayload := `[
		{
			"@context": "https://schema.org",
			"@type": "Recipe",
			"name": "Guacamole Tradicional",
			"recipeYield": "2 raciones",
			"image": ["https://example.com/guacamole.jpg"],
			"recipeIngredient": [
				"2 aguacates maduros",
				"1 lima",
				"50 g cebolla"
			],
			"recipeInstructions": [
				{"@type": "HowToStep", "text": "Chafar los aguacates."},
				{"@type": "HowToStep", "text": "Añadir lima y cebolla."}
			]
		}
	]`

	recipes, err := LoadRecipesFromJSON(strings.NewReader(jsonPayload))
	if err != nil {
		t.Fatalf("LoadRecipesFromJSON falló: %v", err)
	}

	if len(recipes) != 1 {
		t.Fatalf("Esperaba 1 receta, obtenidas %d", len(recipes))
	}

	r := recipes[0]
	if r.Image != "https://example.com/guacamole.jpg" {
		t.Errorf("Imagen esperada URL, obtenida %s", r.Image)
	}
	if r.Yield != 2 {
		t.Errorf("Raciones esperadas 2, obtenidas %d", r.Yield)
	}
	if len(r.Ingredients) != 3 {
		t.Errorf("Ingredientes parseados esperados 3, obtenidos %d", len(r.Ingredients))
	}
	if len(r.Instructions) != 2 {
		t.Errorf("Instrucciones esperadas 2, obtenidas %d", len(r.Instructions))
	}
}
