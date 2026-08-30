package model

// Nutrition contiene los macronutrientes y energía por 100g de alimento
type Nutrition struct {
	Calories float64 `json:"calories"` // kcal por 100g
	Protein  float64 `json:"protein"`  // gramos
	Fat      float64 `json:"fat"`      // gramos
	Carbs    float64 `json:"carbs"`    // gramos
	Fiber    float64 `json:"fiber"`    // gramos
	Sugars   float64 `json:"sugars"`   // gramos
	Sodium   float64 `json:"sodium"`   // miligramos
}

// PrimaryFood representa un alimento primario / genérico y sus bondades
type PrimaryFood struct {
	ID                 string             `json:"id"`                 // Identificador canónico (ej: "brocoli", "avena_hojuelas")
	Name               string             `json:"name"`               // Nombre en español (ej: "Brócoli")
	ScientificName     string             `json:"scientificName"`     // Nombre botánico/científico
	Category           string             `json:"category"`           // "vegetable", "fruit", "cereal", "legume", "nut", "oil", "spice", "fish", "meat", "dairy"
	Aliases            []string           `json:"aliases"`            // Sinónimos / variantes para Smart Match
	NutritionPer100g   Nutrition          `json:"nutritionPer100g"`   // Macros por 100g
	Micronutrients     map[string]float64 `json:"micronutrients"`     // Vitaminas y minerales clave (ej: "vitamina_c_mg": 89.2)
	BioactiveCompounds []string           `json:"bioactiveCompounds"` // Compuestos bioactivos (Sulforafano, Licopeno, Curcumina...)
	Benefits           []string           `json:"benefits"`           // Bondades para la salud (bullet points destacados)
	Synergies          []string           `json:"synergies"`          // Sinergias culinarias y de absorción
	Tags               []string           `json:"tags"`               // Etiquetas ("#antioxidante", "#prebiotico", "#antiinflamatorio")
	DataSource         string             `json:"dataSource"`         // Origen de datos ("BEDCA", "USDA", "CURATED", "AI_ENRICHED")
}

// SearchFilter representa los parámetros de búsqueda y filtrado
type SearchFilter struct {
	Query    string `json:"query"`
	Category string `json:"category"`
	Tag      string `json:"tag"`
	Limit    int    `json:"limit"`
	Offset   int    `json:"offset"`
}
