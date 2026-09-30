package planner

import (
	"testing"
	"time"

	"github.com/josejuanmontiel/primaryfoods/internal/model"
)

func TestMenuPlanner(t *testing.T) {
	planner := NewMenuPlanner(nil)

	recipes := []model.Recipe{
		{
			ID:   "rec-1",
			Slug: "lentejas-estofadas",
			Name: "Lentejas Estofadas de la Abuela",
			Ingredients: []model.RecipeIngredient{
				{Name: "Lentejas"}, {Name: "Zanahoria"}, {Name: "Puerro"},
			},
		},
		{
			ID:   "rec-2",
			Slug: "salmon-al-horno",
			Name: "Salmón al Horno con Verduras",
			Ingredients: []model.RecipeIngredient{
				{Name: "Salmón"}, {Name: "Espinacas"}, {Name: "Aceite de Oliva"},
			},
		},
		{
			ID:   "rec-3",
			Slug: "garbanzos-con-espinacas",
			Name: "Garbanzos con Espinacas",
			Ingredients: []model.RecipeIngredient{
				{Name: "Garbanzos"}, {Name: "Espinacas"}, {Name: "Ajo"},
			},
		},
		{
			ID:   "rec-4",
			Slug: "ensalada-tomate",
			Name: "Ensalada de Tomate y Ventresca",
			Ingredients: []model.RecipeIngredient{
				{Name: "Tomate"}, {Name: "Bonito"}, {Name: "Aceite de Oliva"},
			},
		},
	}

	expDate := time.Now().AddDate(0, 0, 2).Format("2006-01-02")
	pantry := []model.PantryItem{
		{
			ProductCode: "001",
			ProductName: "Espinacas frescas",
			ShelfClass:  "critical", // Próximo a caducar
			ExpiryDate:  &expDate,
		},
	}

	seasonal := []model.SeasonalItem{
		{
			FoodName:     "Tomate",
			Category:     "vegetable",
			Month:        7,
			Availability: model.AvailabilityPeak,
		},
		{
			FoodName:     "Espinacas",
			Category:     "vegetable",
			Month:        7,
			Availability: model.AvailabilityAvailable,
		},
	}

	plan, err := planner.GenerateWeeklyPlan(recipes, pantry, seasonal, model.MenuGenerationOptions{
		Year:             2026,
		Week:             28, // Julio
		PrioritizePantry: true,
	})

	if err != nil {
		t.Fatalf("unexpected error generating plan: %v", err)
	}

	if plan.TotalMeals != 14 {
		t.Errorf("expected 14 meals (7 days x 2), got %d", plan.TotalMeals)
	}

	if plan.PantrySavings == 0 {
		t.Errorf("expected pantry savings with expiring espinacas, got 0")
	}

	if plan.SeasonalMatches == 0 {
		t.Errorf("expected seasonal matches, got 0")
	}

	t.Logf("Plan generated: %d meals, Legumes: %d, Fish: %d, Seasonal: %d, Savings: %d",
		plan.TotalMeals, plan.LegumeDays, plan.FishDays, plan.SeasonalMatches, plan.PantrySavings)
}
