package model

// PantryItem representa un producto o ingrediente en la despensa del usuario (proveniente de Buscatureceta)
type PantryItem struct {
	ID          int64   `json:"id"`
	ProductCode string  `json:"productCode"`
	ProductName string  `json:"productName"`
	Amount      float64 `json:"amount"`
	Unit        string  `json:"unit"`
	PantryZone  string  `json:"pantryZone"` // "food", "non_food", etc.
	ExpiryDate  *string `json:"expiryDate,omitempty"`
	OpenedAt    *string `json:"openedAt,omitempty"`
	IsEstimated bool    `json:"isEstimated,omitempty"`
	ShelfClass  string  `json:"shelfClass,omitempty"` // "long", "medium", "short"
}

// DiaryItem representa un plato dentro de una comida del diario
type DiaryItem struct {
	Course      string                 `json:"course"` // "appetizer", "starter", "main", "dessert", "drink", "snack"
	Type        string                 `json:"type"`   // "recipe", "product", "custom_macros", "photo"
	Name        string                 `json:"name"`
	RecipeID    *int64                 `json:"recipeId,omitempty"`
	ProductCode *string                `json:"productCode,omitempty"`
	Servings    float64                `json:"servings"`
	Status      string                 `json:"status"` // "planned", "consumed", "skipped"
	Nutrition   map[string]interface{} `json:"nutrition,omitempty"`
}

// DiaryEntry representa una comida planificada o consumida en la agenda
type DiaryEntry struct {
	ID        int64       `json:"id"`
	Date      string      `json:"date"`     // "YYYY-MM-DD"
	MealType  string      `json:"mealType"` // "breakfast", "lunch", "dinner", etc.
	Status    string      `json:"status"`   // "planned", "consumed"
	AteAtHome bool        `json:"ate_at_home"`
	Version   int         `json:"version"`
	Items     []DiaryItem `json:"items"`
}

// DiaryVersion representa una snapshot histórica de decisión en el diario
type DiaryVersion struct {
	ID            int64       `json:"id"`
	DiaryEntryID  int64       `json:"diaryEntryId"`
	Date          string      `json:"date"`
	MealType      string      `json:"mealType"`
	Action        string      `json:"action"` // "plan_created", "plan_adjusted", "consumed", "deleted"
	VersionNumber int         `json:"versionNumber"`
	Status        string      `json:"status"`
	AteAtHome     bool        `json:"ate_at_home"`
	Items         []DiaryItem `json:"items"`
	Timestamp     string      `json:"timestamp"`
	Reason        string      `json:"reason,omitempty"`
}

// BuscaturecetaPackage es la estructura unificada de exportación/importación entre Buscatureceta y PrimaryFoods
type BuscaturecetaPackage struct {
	SchemaVersion  string         `json:"schemaVersion"`
	App            string         `json:"app"`
	ExportedAt     string         `json:"exportedAt"`
	Pantry         []PantryItem   `json:"pantry"`
	Diary          []DiaryEntry   `json:"diary"`
	DiaryVersions  []DiaryVersion `json:"diaryVersions"`
	MealTemplates  []interface{}  `json:"mealTemplates,omitempty"`
	CustomProducts []interface{}  `json:"customProducts,omitempty"`
}
