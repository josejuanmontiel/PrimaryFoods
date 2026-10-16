package planner

import (
	"testing"

	"github.com/josejuanmontiel/primaryfoods/internal/model"
)

func TestAnalyzeDiaryDeficiencies(t *testing.T) {
	// Rango de semana 42 de 2026: 2026-10-12 a 2026-10-18
	entries := []model.DiaryEntry{
		{
			ID:       1,
			Date:     "2026-10-12",
			MealType: "lunch",
			Status:   "consumed",
			Items: []model.DiaryItem{
				{Name: "Pollo a la plancha con ensalada de tomate", Course: "main", Status: "consumed"},
			},
		},
		{
			ID:       2,
			Date:     "2026-10-13",
			MealType: "lunch",
			Status:   "consumed",
			Items: []model.DiaryItem{
				{Name: "Lentejas con verduras", Course: "main", Status: "consumed"},
			},
		},
		{
			ID:       3,
			Date:     "2026-10-14",
			MealType: "lunch",
			Status:   "consumed",
			Items: []model.DiaryItem{
				{Name: "Merluza al horno", Course: "main", Status: "consumed"}, // Pescado blanco
			},
		},
		{
			ID:       4,
			Date:     "2026-10-15",
			MealType: "dinner",
			Status:   "skipped", // Plato saltado, no debe contar
			Items: []model.DiaryItem{
				{Name: "Sardinas asadas", Course: "main", Status: "skipped"},
			},
		},
	}

	summary := AnalyzeDiaryDeficiencies(entries, 2026, 42, 3, 3)

	if summary.TotalConsumedMeals != 3 {
		t.Errorf("Esperaba 3 comidas consumidas, obtenido %d", summary.TotalConsumedMeals)
	}

	if summary.SkippedMeals != 1 {
		t.Errorf("Esperaba 1 comida omitida, obtenido %d", summary.SkippedMeals)
	}

	if summary.LegumeMeals != 1 {
		t.Errorf("Esperaba 1 comida con legumbres, obtenido %d", summary.LegumeMeals)
	}

	if summary.LegumeDeficit != 2 {
		t.Errorf("Esperaba déficit de legumbres = 2 (meta 3 - 1 consumida), obtenido %d", summary.LegumeDeficit)
	}

	if summary.FishMeals != 1 {
		t.Errorf("Esperaba 1 comida con pescado, obtenido %d", summary.FishMeals)
	}

	if summary.FishDeficit != 2 {
		t.Errorf("Esperaba déficit de pescado = 2 (meta 3 - 1 consumida), obtenido %d", summary.FishDeficit)
	}

	// Sardinas fue skipped, por lo que oily fish = 0, déficit = 1
	if summary.OilyFishMeals != 0 {
		t.Errorf("Esperaba 0 comidas de pescado azul consumidas, obtenido %d", summary.OilyFishMeals)
	}
	if summary.OilyFishDeficit != 1 {
		t.Errorf("Esperaba déficit de pescado azul = 1, obtenido %d", summary.OilyFishDeficit)
	}

	if len(summary.Deficiencies) < 3 {
		t.Errorf("Esperaba al menos 3 descripciones de carencias, obtenido %d", len(summary.Deficiencies))
	}
}

func TestPlanCompensationFeedbackLoop(t *testing.T) {
	p := NewMenuPlanner(nil)

	recipes := []model.Recipe{
		{
			ID:   "rec-1",
			Slug: "garbanzos-espinacas",
			Name: "Garbanzos con Espinacas",
			Ingredients: []model.RecipeIngredient{
				{Name: "Garbanzos"}, {Name: "Espinacas"},
			},
		},
		{
			ID:   "rec-2",
			Slug: "lentejas-verduras",
			Name: "Lentejas estofadas con verduras",
			Ingredients: []model.RecipeIngredient{
				{Name: "Lentejas"}, {Name: "Zanahoria"},
			},
		},
		{
			ID:   "rec-3",
			Slug: "salmon-plancha",
			Name: "Salmón a la plancha con limón",
			Ingredients: []model.RecipeIngredient{
				{Name: "Salmón"},
			},
		},
		{
			ID:   "rec-4",
			Slug: "ensalada-mixta",
			Name: "Ensalada Mixta de Lechuga y Tomate",
			Ingredients: []model.RecipeIngredient{
				{Name: "Lechuga"}, {Name: "Tomate"},
			},
		},
	}

	// Carencias previas: Faltaron 2 raciones de legumbres y 1 de pescado azul
	deficiencies := &model.DiaryNutritionalSummary{
		Year:            2026,
		Week:            41,
		LegumeDeficit:   2,
		FishDeficit:     1,
		OilyFishDeficit: 1,
	}

	plan, err := p.GenerateWeeklyPlan(recipes, nil, nil, model.MenuGenerationOptions{
		Year:                   2026,
		Week:                   42,
		TargetLegumeDays:       3,
		TargetFishDays:         2,
		CompensateDeficiencies: deficiencies,
	})

	if err != nil {
		t.Fatalf("Error generando menú con compensación: %v", err)
	}

	if len(plan.DeficienciesCompensated) == 0 {
		t.Errorf("Esperaba notas de carencias compensadas en el plan semanal")
	}

	// Como aumentamos el target de legumbres de 3 a 5 debido al déficit, debería programar más legumbres
	if plan.LegumeDays < 4 {
		t.Errorf("Esperaba que planificara al menos 4 días de legumbres para compensar, obtenido %d", plan.LegumeDays)
	}

	// Salmón es pescado azul y debería haber sido priorizado
	if plan.OilyFishDays < 1 {
		t.Errorf("Esperaba al menos 1 día de pescado azul programado, obtenido %d", plan.OilyFishDays)
	}
}
