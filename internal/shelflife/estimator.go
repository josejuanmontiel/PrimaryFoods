package shelflife

import (
	"strings"
	"time"
)

// ShelfClass clasifica los alimentos según su horizonte de vida útil
type ShelfClass string

const (
	ClassLong   ShelfClass = "long"   // > 180 días (arroz, pasta, legumbres secas, conservas, especias)
	ClassMedium ShelfClass = "medium" // 30 - 180 días (bricks, snacks, quesos, congelados)
	ClassShort  ShelfClass = "short"  // < 30 días (frescos, carnes, pescados, lácteos refrigerados)
)

// Rule regla de estimación
type Rule struct {
	Match      []string
	Days       int
	ShelfClass ShelfClass
	Label      string
}

// Estimate resultado de estimación de vida útil
type Estimate struct {
	Days                int        `json:"days"`
	ShelfClass          ShelfClass `json:"shelfClass"`
	Reliability         string     `json:"reliability"` // "high", "medium", "fallback"
	RuleApplied         string     `json:"ruleApplied"`
	SuggestedExpiryDate string     `json:"suggestedExpiryDate"`
}

var offCategoryRules = []Rule{
	{Match: []string{"canned", "tinned", "conserva"}, Days: 1095, ShelfClass: ClassLong, Label: "Conserva / Enlatado"},
	{Match: []string{"spices", "salts", "vinegars", "sugars", "honey"}, Days: 1095, ShelfClass: ClassLong, Label: "Condimentos y Especias"},
	{Match: []string{"pastas", "dry-pasta", "durum-wheat-pasta"}, Days: 730, ShelfClass: ClassLong, Label: "Pasta seca"},
	{Match: []string{"rices", "cereal-grains", "cereal-flakes", "flours"}, Days: 730, ShelfClass: ClassLong, Label: "Arroz / Cereal seco"},
	{Match: []string{"legumes", "pulses", "dried-legumes"}, Days: 730, ShelfClass: ClassLong, Label: "Legumbre seca"},
	{Match: []string{"plant-oils", "olive-oils", "vegetable-oils"}, Days: 540, ShelfClass: ClassLong, Label: "Aceite"},
	{Match: []string{"frozen-foods", "ice-creams"}, Days: 365, ShelfClass: ClassMedium, Label: "Congelado"},
	{Match: []string{"plant-based-beverages", "uht-milks", "broths", "dehydrated-soups", "tomato-sauces"}, Days: 270, ShelfClass: ClassMedium, Label: "Brick / UHT / Salsa"},
	{Match: []string{"biscuits", "cakes", "cereal-bars", "chocolate", "cocoa", "snacks"}, Days: 210, ShelfClass: ClassMedium, Label: "Snacks / Galletas"},
	{Match: []string{"cheeses", "cured-cheeses", "charcuterie"}, Days: 75, ShelfClass: ClassMedium, Label: "Queso / Curado"},
	{Match: []string{"fermented-milk-products", "yogurts", "desserts"}, Days: 28, ShelfClass: ClassShort, Label: "Yogur / Postre lácteo"},
	{Match: []string{"eggs"}, Days: 28, ShelfClass: ClassShort, Label: "Huevos"},
	{Match: []string{"fresh-vegetables", "vegetables", "fruits", "salads"}, Days: 7, ShelfClass: ClassShort, Label: "Fruta / Verdura fresca"},
	{Match: []string{"prepared-meats", "meats", "poultry"}, Days: 5, ShelfClass: ClassShort, Label: "Carne fresca"},
	{Match: []string{"seafood", "fishes"}, Days: 3, ShelfClass: ClassShort, Label: "Pescado fresco"},
}

var bedcaCategoryRules = map[string]struct {
	Days       int
	ShelfClass ShelfClass
	Label      string
}{
	"legume":    {Days: 730, ShelfClass: ClassLong, Label: "Legumbre seca"},
	"cereal":    {Days: 365, ShelfClass: ClassLong, Label: "Cereal / Grano"},
	"spice":     {Days: 730, ShelfClass: ClassLong, Label: "Especia"},
	"oil":       {Days: 540, ShelfClass: ClassLong, Label: "Aceite"},
	"nut":       {Days: 180, ShelfClass: ClassMedium, Label: "Fruto seco"},
	"dairy":     {Days: 21, ShelfClass: ClassShort, Label: "Lácteo"},
	"vegetable": {Days: 7, ShelfClass: ClassShort, Label: "Verdura fresca"},
	"fruit":     {Days: 7, ShelfClass: ClassShort, Label: "Fruta fresca"},
	"meat":      {Days: 4, ShelfClass: ClassShort, Label: "Carne fresca"},
	"fish":      {Days: 3, ShelfClass: ClassShort, Label: "Pescado fresco"},
	"other":     {Days: 60, ShelfClass: ClassMedium, Label: "Varios"},
}

var nameKeywordRules = []struct {
	Keywords   []string
	Days       int
	ShelfClass ShelfClass
	Label      string
}{
	{Keywords: []string{"lata", "bote", "conserva", "enlatad"}, Days: 1095, ShelfClass: ClassLong, Label: "Conserva detectada"},
	{Keywords: []string{"congelad", "ultracongelad"}, Days: 365, ShelfClass: ClassMedium, Label: "Congelado detectado"},
	{Keywords: []string{"arroz", "pasta", "macarron", "espagueti", "fideo", "garbanzo", "lenteja", "alubia", "harina", "avena"}, Days: 730, ShelfClass: ClassLong, Label: "Cereal / Legumbre detectada"},
	{Keywords: []string{"aceite", "vinagre", "sal", "azucar", "miel"}, Days: 730, ShelfClass: ClassLong, Label: "Básico despensa"},
	{Keywords: []string{"leche", "caldo", "brick", "soja", "bebida"}, Days: 180, ShelfClass: ClassMedium, Label: "Brick / Bebida"},
	{Keywords: []string{"galleta", "barrita", "torta"}, Days: 180, ShelfClass: ClassMedium, Label: "Snack seco"},
	{Keywords: []string{"queso"}, Days: 60, ShelfClass: ClassMedium, Label: "Queso"},
	{Keywords: []string{"yogur", "kefir", "flan", "natilla"}, Days: 28, ShelfClass: ClassShort, Label: "Lácteo refrigerado"},
	{Keywords: []string{"huevo", "huevos"}, Days: 28, ShelfClass: ClassShort, Label: "Huevos"},
	{Keywords: []string{"lechuga", "tomate", "espinaca", "ensalada", "fresc"}, Days: 7, ShelfClass: ClassShort, Label: "Fresco perecedero"},
	{Keywords: []string{"carne", "pollo", "ternera", "cerdo", "burger"}, Days: 5, ShelfClass: ClassShort, Label: "Carne fresca"},
	{Keywords: []string{"pescado", "salmon", "merluza", "atun"}, Days: 3, ShelfClass: ClassShort, Label: "Pescado fresco"},
}

// EstimateShelfLife calcula la vida útil y fecha sugerida para un producto
func EstimateShelfLife(productCode, productName, categoriesTags, primaryCategory string, baseDate time.Time) Estimate {
	if baseDate.IsZero() {
		baseDate = time.Now()
	}

	// 1. Tags jerárquicos de OpenFoodFacts
	if categoriesTags != "" {
		lowerTags := strings.ToLower(categoriesTags)
		for _, rule := range offCategoryRules {
			for _, pat := range rule.Match {
				if strings.Contains(lowerTags, pat) {
					return buildEstimate(rule.Days, rule.ShelfClass, "high", "OFF tag: "+pat, baseDate)
				}
			}
		}
	}

	// 2. Categoría canónica BEDCA
	if primaryCategory != "" {
		if rule, ok := bedcaCategoryRules[strings.ToLower(primaryCategory)]; ok {
			return buildEstimate(rule.Days, rule.ShelfClass, "high", "BEDCA: "+primaryCategory, baseDate)
		}
	}

	// Si el código es 'primary:...' inferir por ID
	if strings.HasPrefix(productCode, "primary:") {
		lowerCode := strings.ToLower(productCode)
		for catKey, rule := range bedcaCategoryRules {
			if strings.Contains(lowerCode, catKey) {
				return buildEstimate(rule.Days, rule.ShelfClass, "high", "BEDCA Food: "+catKey, baseDate)
			}
		}
	}

	// 3. Heurística por nombre
	if productName != "" {
		lowerName := strings.ToLower(productName)
		for _, rule := range nameKeywordRules {
			for _, kw := range rule.Keywords {
				if strings.Contains(lowerName, kw) {
					return buildEstimate(rule.Days, rule.ShelfClass, "medium", "Palabra clave: "+kw, baseDate)
				}
			}
		}
	}

	// 4. Fallback conservador por defecto
	return buildEstimate(60, ClassMedium, "fallback", "Regla general por defecto", baseDate)
}

func buildEstimate(days int, shelfClass ShelfClass, reliability, ruleApplied string, baseDate time.Time) Estimate {
	suggested := baseDate.AddDate(0, 0, days).Format("2006-01-02")
	return Estimate{
		Days:                days,
		ShelfClass:          shelfClass,
		Reliability:         reliability,
		RuleApplied:         ruleApplied,
		SuggestedExpiryDate: suggested,
	}
}
