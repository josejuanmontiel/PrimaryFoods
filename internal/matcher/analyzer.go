package matcher

import (
	"regexp"
	"strings"

	"github.com/josejuanmontiel/primaryfoods/internal/model"
	"github.com/josejuanmontiel/primaryfoods/internal/storage"
)

// UnitRegex detecta prefijos de unidades/medidas comunes en recetas en español e inglés
var measurePrefixRegex = regexp.MustCompile(`(?i)^((\d+([.,]\d+)?\s*)?(g|gr|kg|kilo|kilos|ml|l|litro|litros|taza|tazas|cdita|cucharadita|cucharaditas|cda|cucharada|cucharadas|vaso|vasos|diente|dientes|pizca|pizcas|tira|tiras|loncha|lonchas|hoja|hojas|bote|botes|lata|latas|paquete|paquetes|tablespoon|tablespoons|teaspoon|teaspoons|cup|cups|clove|cloves|slice|slices)\s+(de\s+)?)`)

// CleanIngredientName limpia un nombre de ingrediente eliminando unidades iniciales, notas entre paréntesis y stopwords comunes
func CleanIngredientName(raw string) string {
	clean := strings.TrimSpace(raw)
	// Quitar texto entre paréntesis: ej "Relleno (aguacate, cangrejo...)" -> "Relleno"
	if idx := strings.Index(clean, "("); idx > 0 {
		clean = strings.TrimSpace(clean[:idx])
	}

	// Quitar cantidades iniciales tipo "1 ", "1/2 ", "¼ ", "200g "
	clean = regexp.MustCompile(`^[\d¼½¾⅓⅔⅛/\.,\-]+\s*`).ReplaceAllString(clean, "")

	// Quitar prefijos de medida tipo "taza de...", "cucharada de...", "tira de..."
	clean = measurePrefixRegex.ReplaceAllString(clean, "")

	// Quitar prefijos "de "
	clean = regexp.MustCompile(`(?i)^de\s+`).ReplaceAllString(clean, "")

	// Normalización general (sin tildes, minúsculas)
	norm := Normalize(clean)

	// Reglas de lematización directa y corrección de typos frecuentes
	switch norm {
	case "cebollotas":
		return "cebolleta"
	case "lombrada":
		return "lombarda"
	case "tomates", "tomates maduros":
		return "tomate"
	case "zanahorias":
		return "zanahoria"
	case "huevos":
		return "huevo"
	case "pimientos verdes", "pimiento verde":
		return "pimiento"
	case "pimientos rojos", "pimiento rojo":
		return "pimiento"
	case "anacardos":
		return "anacardo"
	case "nori", "algas nori", "hojas nori":
		return "alga nori"
	case "anchovies in salt":
		return "anchoas"
	case "small dried red pepper":
		return "guindilla"
	case "garlic cloves":
		return "ajo"
	case "ripe tomatoes for sauce":
		return "tomate"
	case "black olives":
		return "aceitunas negras"
	case "small capers":
		return "alcaparras"
	case "extra virgin olive oil":
		return "aceite oliva virgen extra"
	case "oregano":
		return "oregano"
	case "coliflor coco comino":
		return "coliflor"
	}

	return norm
}

// RecipeAnalyzer coordina el análisis profundo de recetas contra Alimentos Primarios y Productos Comerciales
type RecipeAnalyzer struct {
	storage *storage.Storage
	matcher *RecipeMatcher
}

// NewRecipeAnalyzer crea una nueva instancia del analizador
func NewRecipeAnalyzer(store *storage.Storage, foods []model.PrimaryFood) *RecipeAnalyzer {
	return &RecipeAnalyzer{
		storage: store,
		matcher: NewMatcher(foods),
	}
}

// AnalyzeRecipe deconstruye cada ingrediente de la receta y busca equivalentes primarios y comerciales
func (ra *RecipeAnalyzer) AnalyzeRecipe(recipe model.Recipe) (model.RecipeAnalysis, error) {
	analysis := model.RecipeAnalysis{
		Recipe:           recipe,
		TotalIngredients: len(recipe.Ingredients),
	}

	for _, ing := range recipe.Ingredients {
		cleaned := CleanIngredientName(ing.Name)
		if cleaned == "" {
			cleaned = Normalize(ing.Name)
		}

		res := model.IngredientResolution{
			OriginalText:   ing.Name,
			NormalizedName: cleaned,
			Amount:         ing.Amount,
			Unit:           ing.Unit,
			Status:         "unresolved",
		}

		// 1. Revisar si hay un mapping persistente previo (memoria aprendida)
		if mapping, err := ra.storage.GetIngredientMapping(cleaned); err == nil && mapping != nil {
			if food, err := ra.storage.GetByID(mapping.CanonicalPrimaryID); err == nil && food != nil {
				res.PrimaryFood = food
				res.Status = "primary_match"
			}
		}

		// 2. Si no, buscar en catálogo de Alimentos Primarios
		if res.PrimaryFood == nil {
			primaryID := ra.matcher.findPrimaryID(cleaned)
			if primaryID != "" {
				if food, err := ra.storage.GetByID(primaryID); err == nil && food != nil {
					res.PrimaryFood = food
					res.Status = "primary_match"
				}
			}
		}

		// 3. Si aún no o adicionalmente, buscar en Alimentos Primarios por Search
		if res.PrimaryFood == nil {
			foods, err := ra.storage.Search(cleaned, 1)
			if err == nil && len(foods) > 0 {
				res.PrimaryFood = &foods[0]
				res.Status = "primary_match"
			}
		}

		// 4. Buscar alternativas en productos comerciales (OpenFoodFacts)
		// Términos de búsqueda: el nombre limpio, o variantes clave (ej: "macarrones", "masa quebrada", "obleas")
		searchTerm := cleaned
		if res.PrimaryFood != nil && (cleaned == "pasta" || cleaned == "queso") {
			searchTerm = ing.Name // mantener especificidad comercial
		}

		commProducts, err := ra.storage.SearchCommercialProducts(searchTerm, 4)
		if err == nil && len(commProducts) > 0 {
			res.CommercialAlternatives = commProducts
			if res.Status == "unresolved" {
				res.Status = "commercial_match"
			}
		}

		// Si sigue unresolved, intentar buscar por tokens principales
		if res.Status == "unresolved" {
			words := strings.Fields(cleaned)
			for _, w := range words {
				if len(w) >= 4 {
					subComm, _ := ra.storage.SearchCommercialProducts(w, 3)
					if len(subComm) > 0 {
						res.CommercialAlternatives = subComm
						res.Status = "commercial_match"
						break
					}
				}
			}
		}

		// Contadores
		switch res.Status {
		case "primary_match":
			analysis.PrimaryCount++
		case "commercial_match":
			analysis.CommercialCount++
		default:
			analysis.UnresolvedCount++
		}

		analysis.Resolutions = append(analysis.Resolutions, res)
	}

	if analysis.TotalIngredients > 0 {
		resolved := analysis.PrimaryCount + analysis.CommercialCount
		analysis.MatchPercentage = (float64(resolved) / float64(analysis.TotalIngredients)) * 100.0
	}

	return analysis, nil
}
