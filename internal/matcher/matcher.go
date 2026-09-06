package matcher

import (
	"sort"
	"strings"
	"unicode"

	"github.com/josejuanmontiel/primaryfoods/internal/model"
)

// MatchOptions define los criterios para filtrar y ordenar recetas según la despensa
type MatchOptions struct {
	ReadyToCookOnly    bool    // Devolver únicamente recetas 100% listas para preparar
	MaxMissing         int     // Máximo de ingredientes faltantes permitidos (-1 para sin límite)
	MinMatchPercentage float64 // Porcentaje mínimo de coincidencia (0-100)
}

// RecipeMatcher evalúa la disponibilidad de recetas cruzando despensa y catálogo de alimentos primarios
type RecipeMatcher struct {
	primaryFoods []model.PrimaryFood
	// Mapa inverso: alias/nombre normalizado -> PrimaryFood ID
	aliasToID map[string]string
}

// NewMatcher inicializa el motor de matching con el catálogo opcional de PrimaryFoods
func NewMatcher(foods []model.PrimaryFood) *RecipeMatcher {
	aliasMap := make(map[string]string)
	for _, f := range foods {
		normName := Normalize(f.Name)
		aliasMap[normName] = f.ID
		for _, alias := range f.Aliases {
			normAlias := Normalize(alias)
			aliasMap[normAlias] = f.ID
		}
	}
	return &RecipeMatcher{
		primaryFoods: foods,
		aliasToID:    aliasMap,
	}
}

// MatchRecipes evalúa una lista de recetas contra los productos de la despensa
func (m *RecipeMatcher) MatchRecipes(recipes []model.Recipe, pantry []model.PantryItem, opts MatchOptions) []model.RecipeMatch {
	// Pre-normalizar los productos de la despensa para comparaciones O(1)/rápidas
	type normPantryItem struct {
		originalName string
		normalized   string
		primaryID    string
	}
	var normPantry []normPantryItem
	for _, p := range pantry {
		norm := Normalize(p.ProductName)
		if norm == "" {
			continue
		}
		primaryID := m.findPrimaryID(norm)
		normPantry = append(normPantry, normPantryItem{
			originalName: p.ProductName,
			normalized:   norm,
			primaryID:    primaryID,
		})
	}

	var results []model.RecipeMatch

	for _, recipe := range recipes {
		if len(recipe.Ingredients) == 0 {
			continue
		}

		var available []string
		var missing []string

		for _, ing := range recipe.Ingredients {
			normIng := Normalize(ing.Name)
			if normIng == "" {
				continue
			}

			// Ingredientes básicos universales (ej: agua, sal) se consideran disponibles
			if isUniversalPantryItem(normIng) {
				available = append(available, ing.Name)
				continue
			}

			ingPrimaryID := m.findPrimaryID(normIng)
			found := false

			for _, p := range normPantry {
				// 1. Coincidencia por ID canónico de Alimento Primario
				if ingPrimaryID != "" && p.primaryID != "" && ingPrimaryID == p.primaryID {
					found = true
					break
				}
				// 2. Coincidencia de texto directo o por subcadena
				if strings.Contains(p.normalized, normIng) || strings.Contains(normIng, p.normalized) {
					found = true
					break
				}
				// 3. Coincidencia por tokens relevantes (raíces de palabras)
				if tokensMatch(normIng, p.normalized) {
					found = true
					break
				}
			}

			if found {
				available = append(available, ing.Name)
			} else {
				missing = append(missing, ing.Name)
			}
		}

		total := len(recipe.Ingredients)
		availCount := len(available)
		missingCount := len(missing)

		var pct float64
		if total > 0 {
			pct = (float64(availCount) / float64(total)) * 100.0
		}

		isReady := missingCount == 0

		// Aplicar filtros
		if opts.ReadyToCookOnly && !isReady {
			continue
		}
		if opts.MaxMissing >= 0 && missingCount > opts.MaxMissing {
			continue
		}
		if opts.MinMatchPercentage > 0 && pct < opts.MinMatchPercentage {
			continue
		}

		results = append(results, model.RecipeMatch{
			Recipe:               recipe,
			TotalIngredients:     total,
			AvailableCount:       availCount,
			MissingCount:         missingCount,
			AvailableIngredients: available,
			MissingIngredients:   missing,
			MatchPercentage:      pct,
			IsReadyToCook:        isReady,
		})
	}

	// Ordenar:
	// 1. IsReadyToCook (true primero)
	// 2. MissingCount (menor a mayor)
	// 3. MatchPercentage (mayor a menor)
	// 4. Nombre alfabéticamente
	sort.Slice(results, func(i, j int) bool {
		if results[i].IsReadyToCook != results[j].IsReadyToCook {
			return results[i].IsReadyToCook
		}
		if results[i].MissingCount != results[j].MissingCount {
			return results[i].MissingCount < results[j].MissingCount
		}
		if results[i].MatchPercentage != results[j].MatchPercentage {
			return results[i].MatchPercentage > results[j].MatchPercentage
		}
		return results[i].Recipe.Name < results[j].Recipe.Name
	})

	return results
}

func (m *RecipeMatcher) findPrimaryID(normalized string) string {
	if id, ok := m.aliasToID[normalized]; ok {
		return id
	}
	// Probar sub-frases
	for alias, id := range m.aliasToID {
		if len(alias) >= 4 && (strings.Contains(normalized, alias) || strings.Contains(alias, normalized)) {
			return id
		}
	}
	return ""
}

// Stopwords y términos culinarios comunes que no aportan valor de ingrediente
var stopWords = map[string]bool{
	"de": true, "del": true, "el": true, "la": true, "los": true, "las": true,
	"en": true, "al": true, "con": true, "sin": true, "un": true, "una": true,
	"fresco": true, "fresca": true, "frescos": true, "frescas": true,
	"picado": true, "picada": true, "troceado": true, "troceada": true,
	"rallado": true, "rallada": true, "cocido": true, "cocida": true,
	"maduro": true, "madura": true, "virgen": true, "extra": true,
	"opcional": true, "gusto": true, "para": true, "servir": true,
}

// Normalize estandariza cadenas eliminando tildes, signos y palabras vacías
func Normalize(text string) string {
	text = strings.ToLower(text)
	var b strings.Builder
	for _, r := range text {
		switch r {
		case 'á', 'à', 'ä', 'â':
			b.WriteRune('a')
		case 'é', 'è', 'ë', 'ê':
			b.WriteRune('e')
		case 'í', 'ì', 'ï', 'î':
			b.WriteRune('i')
		case 'ó', 'ò', 'ö', 'ô':
			b.WriteRune('o')
		case 'ú', 'ù', 'ü', 'û':
			b.WriteRune('u')
		default:
			if unicode.IsLetter(r) || unicode.IsDigit(r) || r == ' ' {
				b.WriteRune(r)
			} else {
				b.WriteRune(' ')
			}
		}
	}

	rawWords := strings.Fields(b.String())
	var cleanWords []string
	for _, w := range rawWords {
		if !stopWords[w] && len(w) > 1 {
			cleanWords = append(cleanWords, w)
		}
	}
	return strings.Join(cleanWords, " ")
}

// tokensMatch compara si al menos dos palabras significativas coinciden o si una palabra clave coincide exactamente
func tokensMatch(a, b string) bool {
	wordsA := strings.Fields(a)
	wordsB := strings.Fields(b)

	matchCount := 0
	for _, wa := range wordsA {
		if len(wa) < 3 {
			continue
		}
		for _, wb := range wordsB {
			if wa == wb || (len(wa) >= 5 && strings.HasPrefix(wb, wa[:len(wa)-1])) {
				matchCount++
				if matchCount >= 1 && (len(wa) >= 5 || len(wordsA) == 1) {
					return true
				}
			}
		}
	}
	return matchCount >= 2
}

func isUniversalPantryItem(norm string) bool {
	return norm == "agua" || norm == "sal" || norm == "pimienta"
}
