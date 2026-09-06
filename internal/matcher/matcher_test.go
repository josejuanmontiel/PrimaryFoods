package matcher

import (
	"testing"

	"github.com/josejuanmontiel/primaryfoods/internal/model"
)

func TestRecipeMatcher_MatchRecipes(t *testing.T) {
	primaryFoods := []model.PrimaryFood{
		{
			ID:      "brocoli",
			Name:    "Brócoli",
			Aliases: []string{"broculi", "brecol"},
		},
		{
			ID:      "quinoa",
			Name:    "Quinoa",
			Aliases: []string{"quinua"},
		},
		{
			ID:      "aceite_oliva_virgen_extra",
			Name:    "Aceite de Oliva Virgen Extra",
			Aliases: []string{"aove", "aceite de oliva"},
		},
		{
			ID:      "tofu",
			Name:    "Tofu",
			Aliases: []string{"tofu firme", "tofu sedoso"},
		},
	}

	matcher := NewMatcher(primaryFoods)

	// Recetas de prueba
	recipes := []model.Recipe{
		{
			ID:   "rec-1",
			Slug: "brocoli-quinoa",
			Name: "Brócoli al vapor con Quinoa",
			Ingredients: []model.RecipeIngredient{
				{Name: "Brócoli fresco", Amount: 200, Unit: "g"},
				{Name: "Quinoa cocida", Amount: 150, Unit: "g"},
				{Name: "Agua", Amount: 500, Unit: "ml"},
			},
		},
		{
			ID:   "rec-2",
			Slug: "salteado-tofu",
			Name: "Salteado de Tofu con Brócoli",
			Ingredients: []model.RecipeIngredient{
				{Name: "Brócoli", Amount: 200, Unit: "g"},
				{Name: "Tofu", Amount: 150, Unit: "g"},
				{Name: "Salsa de soja", Amount: 20, Unit: "ml"},
			},
		},
		{
			ID:   "rec-3",
			Slug: "salmon-horno",
			Name: "Salmón al Horno con Patatas",
			Ingredients: []model.RecipeIngredient{
				{Name: "Salmón fresco", Amount: 300, Unit: "g"},
				{Name: "Patatas", Amount: 200, Unit: "g"},
			},
		},
	}

	// Despensa del usuario: tiene brócoli, quinoa y tofu (no tiene salsa de soja ni salmón)
	pantry := []model.PantryItem{
		{ProductName: "Brócoli Hacendado 500g", Amount: 500, Unit: "g"},
		{ProductName: "Quinoa Real Bio", Amount: 300, Unit: "g"},
		{ProductName: "Tofu Firme Natural", Amount: 200, Unit: "g"},
	}

	// Test 1: Matching general sin filtros
	matches := matcher.MatchRecipes(recipes, pantry, MatchOptions{MaxMissing: -1})
	if len(matches) != 3 {
		t.Fatalf("Esperaba 3 matches evaluados, obtenidos %d", len(matches))
	}

	// La primera debe ser la 100% cocinable (Brócoli con Quinoa)
	m1 := matches[0]
	if m1.Recipe.Slug != "brocoli-quinoa" {
		t.Errorf("La primera receta debería ser brocoli-quinoa, fue %s", m1.Recipe.Slug)
	}
	if !m1.IsReadyToCook {
		t.Errorf("brocoli-quinoa debería ser IsReadyToCook == true")
	}
	if m1.MissingCount != 0 {
		t.Errorf("brocoli-quinoa debería tener 0 faltantes, tiene %d", m1.MissingCount)
	}

	// La segunda debe ser la que tiene 1 faltante (Salteado de Tofu, le falta salsa de soja)
	m2 := matches[1]
	if m2.Recipe.Slug != "salteado-tofu" {
		t.Errorf("La segunda receta debería ser salteado-tofu, fue %s", m2.Recipe.Slug)
	}
	if m2.MissingCount != 1 {
		t.Errorf("salteado-tofu debería tener 1 faltante, tiene %d (%v)", m2.MissingCount, m2.MissingIngredients)
	}
	if len(m2.MissingIngredients) == 0 || m2.MissingIngredients[0] != "Salsa de soja" {
		t.Errorf("Ingrediente faltante esperado 'Salsa de soja', recibido: %v", m2.MissingIngredients)
	}

	// Test 2: Filtro ReadyToCookOnly
	readyOnly := matcher.MatchRecipes(recipes, pantry, MatchOptions{ReadyToCookOnly: true})
	if len(readyOnly) != 1 {
		t.Fatalf("Esperaba solo 1 receta lista para cocinar, obtenidas %d", len(readyOnly))
	}
	if readyOnly[0].Recipe.Slug != "brocoli-quinoa" {
		t.Errorf("Receta lista inesperada: %s", readyOnly[0].Recipe.Slug)
	}
}

func TestNormalize(t *testing.T) {
	cases := []struct {
		input    string
		expected string
	}{
		{"Brócoli fresco picado", "brocoli"},
		{"Aceite de Oliva Virgen Extra", "aceite oliva"},
		{"PeChUgA de PoLlO en daditos", "pechuga pollo daditos"},
	}

	for _, c := range cases {
		got := Normalize(c.input)
		if got != c.expected {
			t.Errorf("Normalize(%q) = %q; esperaba %q", c.input, got, c.expected)
		}
	}
}
