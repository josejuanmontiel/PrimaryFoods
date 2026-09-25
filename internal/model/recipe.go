package model

// RecipeIngredient representa un ingrediente normalizado de una receta
type RecipeIngredient struct {
	Name         string  `json:"name"`
	Amount       float64 `json:"amount"`
	Unit         string  `json:"unit"`
	OriginalText string  `json:"originalText,omitempty"`
}

// Recipe representa una receta indexada (proveniente de Mealie o de catálogo interno)
type Recipe struct {
	ID           string             `json:"id"`
	Slug         string             `json:"slug"`
	Name         string             `json:"name"`
	Description  string             `json:"description"`
	Yield        int                `json:"yield"`
	Ingredients  []RecipeIngredient `json:"ingredients"`
	Instructions []string           `json:"instructions"`
	Tags         []string           `json:"tags"`
	TotalTime    string             `json:"totalTime,omitempty"`
	Image        string             `json:"image,omitempty"`
	Source       string             `json:"source"` // "mealie", "internal", "generated"
}

// RecipeMatch representa el resultado de evaluar la disponibilidad de una receta contra la despensa
type RecipeMatch struct {
	Recipe               Recipe   `json:"recipe"`
	TotalIngredients     int      `json:"totalIngredients"`
	AvailableCount       int      `json:"availableCount"`
	MissingCount         int      `json:"missingCount"`
	AvailableIngredients []string `json:"availableIngredients"`
	MissingIngredients   []string `json:"missingIngredients"`
	MatchPercentage      float64  `json:"matchPercentage"` // 0.0 a 100.0%
	IsReadyToCook        bool     `json:"isReadyToCook"`    // true si MatchPercentage == 100%
}
