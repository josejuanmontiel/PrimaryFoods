package model

// DiaryNutritionalSummary resume los totales nutricionales observados en las ingestas reales
type DiaryNutritionalSummary struct {
	Year               int      `json:"year"`
	Week               int      `json:"week"`
	StartDate          string   `json:"startDate"`
	EndDate            string   `json:"endDate"`
	TotalConsumedMeals int      `json:"totalConsumedMeals"`
	LegumeMeals        int      `json:"legumeMeals"`
	FishMeals          int      `json:"fishMeals"`
	OilyFishMeals      int      `json:"oilyFishMeals"`
	VegetableMeals     int      `json:"vegetableMeals"`
	SkippedMeals       int      `json:"skippedMeals"`
	EatenOutMeals      int      `json:"eatenOutMeals"`
	Deficiencies       []string `json:"deficiencies"` // Lista descriptiva de carencias detectadas
	LegumeDeficit      int      `json:"legumeDeficit"` // Raciones de legumbres que faltaron para el objetivo
	FishDeficit        int      `json:"fishDeficit"`   // Raciones de pescado que faltaron para el objetivo
	OilyFishDeficit    int      `json:"oilyFishDeficit"`// Raciones de pescado azul que faltaron para el objetivo
}
