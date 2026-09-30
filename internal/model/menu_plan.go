package model

// DailyMealSlot representa la comida de un día
type DailyMealSlot struct {
	Date            string             `json:"date"`            // "YYYY-MM-DD"
	DayOfWeek       string             `json:"dayOfWeek"`       // "Lunes", "Martes"...
	MealType        string             `json:"mealType"`        // "almuerzo", "cena"
	Recipe          Recipe             `json:"recipe"`
	MatchPercentage float64            `json:"matchPercentage"`
	UsedExpiring    []string           `json:"usedExpiring"`    // Ingredientes en riesgo de caducidad aprovechados
	SeasonalFoods   []string           `json:"seasonalFoods"`   // Ingredientes de temporada aprovechados
	DietaryTags     []string           `json:"dietaryTags"`     // "legumbre", "pescado_azul", "verdura"...
	EnergyClass     CookingEnergyClass `json:"energyClass,omitempty"` // zero, minimal, medium, high
	CookingTip      string             `json:"cookingTip,omitempty"`  // Consejo de ahorro de combustible
	Score           float64            `json:"score"`           // Puntuación del algoritmo
}

// WeeklyMenuPlan representa el menú de 7 días generado
type WeeklyMenuPlan struct {
	Year            int             `json:"year"`
	WeekNumber      int             `json:"weekNumber"`      // 1-52
	Month           int             `json:"month"`           // 1-12
	TotalMeals      int             `json:"totalMeals"`      // ej: 14 (7 almuerzos + 7 cenas)
	LegumeDays      int             `json:"legumeDays"`      // Días con legumbres
	FishDays        int             `json:"fishDays"`        // Días con pescado
	OilyFishDays    int             `json:"oilyFishDays"`    // Días con pescado azul
	SeasonalMatches int             `json:"seasonalMatches"` // Recetas con ingredientes de temporada
	PantrySavings   int             `json:"pantrySavings"`   // Ingredientes usados antes de caducar
	ZeroFuelMeals   int             `json:"zeroFuelMeals"`   // Comidas listas sin encender fuego
	MinimalFuelMeals int            `json:"minimalFuelMeals"`// Comidas que solo requieren 1-3 min de agua caliente
	Slots           []DailyMealSlot `json:"slots"`
}

// MenuGenerationOptions define las restricciones y preferencias del usuario
type MenuGenerationOptions struct {
	Year             int  `json:"year"`
	Week             int  `json:"week"`
	PrioritizePantry bool `json:"prioritizePantry"` // Dar más peso a lo que caduca antes
	FuelSavingMode   bool `json:"fuelSavingMode"`   // Modo supervivencia/ahorro energético: prioriza zero/minimal fire
	TargetLegumeDays int  `json:"targetLegumeDays"` // Objetivo recomendado: 3-4
	TargetFishDays   int  `json:"targetFishDays"`   // Objetivo recomendado: 2-3
}

// AnnualMenuPlan representa la proyección anual completa de 52 semanas
type AnnualMenuPlan struct {
	Year             int              `json:"year"`
	TotalWeeks       int              `json:"totalWeeks"`
	TotalMeals       int              `json:"totalMeals"`
	TotalLegumeMeals int              `json:"totalLegumeMeals"`
	TotalFishMeals   int              `json:"totalFishMeals"`
	TotalOilyFishMeals int            `json:"totalOilyFishMeals"`
	ZeroFuelMeals    int              `json:"zeroFuelMeals"`
	MinimalFuelMeals int              `json:"minimalFuelMeals"`
	MinDaysBetweenRepeats int         `json:"minDaysBetweenRepeats"`
	GeneratedAt      string           `json:"generatedAt"`
	Weeks            []WeeklyMenuPlan `json:"weeks"`
}

// AnnualPlanOptions define las opciones para el algoritmo batch de 52 semanas
type AnnualPlanOptions struct {
	Year                  int  `json:"year"`
	MinDaysBetweenRepeats int  `json:"minDaysBetweenRepeats"` // Por defecto: 10 días
	PrioritizePantry      bool `json:"prioritizePantry"`
	FuelSavingMode        bool `json:"fuelSavingMode"`
	TargetLegumeDays      int  `json:"targetLegumeDays"` // 3-4
	TargetFishDays        int  `json:"targetFishDays"`   // 2-3
}
