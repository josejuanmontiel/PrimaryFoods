package importer

import (
	"testing"

	"github.com/josejuanmontiel/primaryfoods/internal/model"
)

func TestSplitIngredients(t *testing.T) {
	text := "Harina de trigo (gluten 15%), agua, salsa de soja (agua, semillas de soja, sal marina), aceite de girasol 0,9%, jengibre. Puede contener apio y mostaza."

	tokens := SplitIngredients(text)
	if len(tokens) != 5 {
		t.Fatalf("Esperaba 5 tokens (descartando trazas), obtenidos %d: %v", len(tokens), tokens)
	}

	if tokens[0] != "Harina de trigo (gluten 15%)" {
		t.Errorf("Token 0 incorrecto: %s", tokens[0])
	}
	if tokens[2] != "salsa de soja (agua, semillas de soja, sal marina)" {
		t.Errorf("Token 2 no respetó los paréntesis: %s", tokens[2])
	}
	if tokens[3] != "aceite de girasol 0,9%" {
		t.Errorf("Token 3 incorrecto: %s", tokens[3])
	}
	if tokens[4] != "jengibre" {
		t.Errorf("Token 4 incorrecto: %s", tokens[4])
	}
}

func TestDeconstructIngredients(t *testing.T) {
	foods := []model.PrimaryFood{
		{ID: "cacao", Name: "Cacao en polvo", Aliases: []string{"pasta de cacao", "cacao"}},
		{ID: "almendra", Name: "Almendra", Aliases: []string{"almendras", "almendra"}},
		{ID: "trigo", Name: "Trigo", Aliases: []string{"harina de trigo", "trigo"}},
	}

	parser := NewLabelParser(foods)

	prod := model.CommercialProduct{
		Code:            "84800001",
		ProductName:     "Chocolate con Almendras",
		Brands:          "Hacendado",
		NovaGroup:       "4",
		IngredientsText: "Pasta de cacao, almendras enteras 25%, azúcar, grasa de palma, emulgente (E-322), colorante (E-102). Puede contener trazas de leche.",
	}

	decon := parser.DeconstructIngredients(prod)

	if decon.MainBase != "cacao" {
		t.Errorf("MainBase esperada 'cacao', obtenida '%s'", decon.MainBase)
	}

	if len(decon.TopBases) < 2 || decon.TopBases[0] != "cacao" || decon.TopBases[1] != "almendra" {
		t.Errorf("TopBases esperadas [cacao, almendra], obtenidas %v", decon.TopBases)
	}

	if decon.DeclaredNoblePercent != 25.0 {
		t.Errorf("DeclaredNoblePercent esperada 25.0, obtenida %.1f", decon.DeclaredNoblePercent)
	}

	if !decon.HasAddedSugars {
		t.Errorf("Debe detectar azúcar añadido")
	}

	if !decon.HasRefinedOils {
		t.Errorf("Debe detectar grasa de palma")
	}

	if len(decon.HighRiskAdditives) != 1 || decon.HighRiskAdditives[0] != "E-102" {
		t.Errorf("Esperaba detectar aditivo de alto riesgo E-102, detectados: %v", decon.HighRiskAdditives)
	}

	if decon.EstimatedPurityScore >= 50.0 {
		t.Errorf("PurityScore de ultraprocesado con E-102, azúcar y palma debe ser bajo (<50), obtenido %.1f", decon.EstimatedPurityScore)
	}
}
