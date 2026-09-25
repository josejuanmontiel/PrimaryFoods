package model

// CommercialProduct representa un producto comercial de OpenFoodFacts
type CommercialProduct struct {
	Code            string   `json:"code"`
	ProductName     string   `json:"productName"`
	Brands          string   `json:"brands,omitempty"`
	NutriscoreGrade string   `json:"nutriscoreGrade,omitempty"`
	NovaGroup       string   `json:"novaGroup,omitempty"`
	IngredientsText string   `json:"ingredientsText,omitempty"`
	CategoriesTags  string   `json:"categoriesTags,omitempty"`
	ImageURL        string   `json:"imageUrl,omitempty"`
	EnergyKcal      float64  `json:"energyKcal"`
	Protein         float64  `json:"protein"`
	Fat             float64  `json:"fat"`
	Carbs           float64  `json:"carbs"`
	Fiber           float64  `json:"fiber"`
	Salt            float64  `json:"salt"`
	TopBases        []string `json:"topBases,omitempty"` // IDs canónicos de alimentos primarios base detectados
}

// IngredientResolution representa el resultado de la deconstrucción y alternativas comerciales de un ingrediente
type IngredientResolution struct {
	OriginalText           string              `json:"originalText"`
	NormalizedName         string              `json:"normalizedName"`
	Amount                 float64             `json:"amount"`
	Unit                   string              `json:"unit"`
	PrimaryFood            *PrimaryFood        `json:"primaryFood,omitempty"`
	CommercialAlternatives []CommercialProduct `json:"commercialAlternatives,omitempty"`
	Status                 string              `json:"status"` // "primary_match", "commercial_match", "unresolved"
}

// RecipeAnalysis representa el análisis completo de una receta y sus ingredientes
type RecipeAnalysis struct {
	Recipe           Recipe                 `json:"recipe"`
	Resolutions      []IngredientResolution `json:"resolutions"`
	TotalIngredients int                    `json:"totalIngredients"`
	PrimaryCount     int                    `json:"primaryCount"`
	CommercialCount  int                    `json:"commercialCount"`
	UnresolvedCount  int                    `json:"unresolvedCount"`
	MatchPercentage  float64                `json:"matchPercentage"` // % resueltos (primarios + comerciales)
}

// IngredientMapping persiste el conocimiento aprendido/asociado de un término a su base primaria
type IngredientMapping struct {
	RawTerm            string   `json:"rawTerm"`
	CanonicalPrimaryID string   `json:"canonicalPrimaryId"`
	CommercialCodes    []string `json:"commercialCodes,omitempty"`
	Confidence         float64  `json:"confidence"`
	Source             string   `json:"source"` // "rule", "ai_curated", "manual"
}

// DeconstructedIngredient representa un ingrediente individual extraído del etiquetado legal
type DeconstructedIngredient struct {
	Position      int      `json:"position"`               // 1 = componente mayoritario, 2, 3...
	RawText       string   `json:"rawText"`                // Texto original en la etiqueta
	CleanName     string   `json:"cleanName"`              // Nombre normalizado
	Percentage    *float64 `json:"percentage,omitempty"`   // Porcentaje % declarado si está disponible (QUID)
	PrimaryFoodID string   `json:"primaryFoodId,omitempty"`// Alimento primario asociado si coincide
	IsAdditive    bool     `json:"isAdditive"`             // Es aditivo (E-number o función tecnológica)
	AdditiveCode  string   `json:"additiveCode,omitempty"` // ej. "E-202" o "E102"
	AdditiveRisk  string   `json:"additiveRisk,omitempty"` // "bajo", "medio", "alto"
}

// LabelDeconstruction contiene el análisis normativo completo del etiquetado de un producto
type LabelDeconstruction struct {
	ProductCode          string                    `json:"productCode"`
	ProductName          string                    `json:"productName"`
	Brands               string                    `json:"brands,omitempty"`
	MainBase             string                    `json:"mainBase"`             // Top 1 ingrediente
	TopBases             []string                  `json:"topBases"`             // Top 1-3 bases primarias
	Ingredients          []DeconstructedIngredient `json:"ingredients"`          // Lista ordenada de mayor a menor peso
	AdditivesCount       int                       `json:"additivesCount"`       // Total aditivos detectados
	HighRiskAdditives    []string                  `json:"highRiskAdditives"`    // Aditivos de riesgo alto detectados
	DeclaredNoblePercent float64                   `json:"declaredNoblePercent"` // Suma de % conocidos de ingredientes nobles
	HasAddedSugars       bool                      `json:"hasAddedSugars"`       // Contiene azúcares añadidos / jarabes
	HasRefinedOils       bool                      `json:"hasRefinedOils"`       // Contiene aceites refinados / palma
	EstimatedPurityScore float64                   `json:"estimatedPurityScore"` // 0.0 - 100.0 métrica de pureza
}

// ProductPrimaryBase representa el enlace relacional entre un producto comercial y un alimento primario base
type ProductPrimaryBase struct {
	ProductCode   string  `json:"productCode"`
	PrimaryFoodID string  `json:"primaryFoodId"`
	Position      int     `json:"position"`
	Percentage    float64 `json:"percentage"`
	IsMainBase    bool    `json:"isMainBase"`
	PurityScore   float64 `json:"purityScore"`
}

// CommercialProductWithBase producto comercial enriquecido con su base primaria y pureza
type CommercialProductWithBase struct {
	Product     CommercialProduct `json:"product"`
	BaseFoodID  string            `json:"baseFoodId"`
	Position    int               `json:"position"`
	Percentage  float64           `json:"percentage"`
	IsMainBase  bool              `json:"isMainBase"`
	PurityScore float64           `json:"purityScore"`
}


