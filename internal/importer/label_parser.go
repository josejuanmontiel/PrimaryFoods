package importer

import (
	"regexp"
	"strconv"
	"strings"
	"unicode"

	"github.com/josejuanmontiel/primaryfoods/internal/matcher"
	"github.com/josejuanmontiel/primaryfoods/internal/model"
)

// Catálogo de aditivos de riesgo alto y medio (extraídos de la normativa y del catálogo de Buscatureceta)
var highRiskAdditives = map[string]bool{
	"e102": true, "e104": true, "e110": true, "e122": true, "e123": true,
	"e124": true, "e127": true, "e129": true, "e171": true, "e214": true,
	"e249": true, "e250": true, "e251": true, "e252": true, "e320": true,
	"e321": true, "e950": true, "e951": true, "e952": true, "e954": true,
	"e955": true, "e621": true,
}

var mediumRiskAdditives = map[string]bool{
	"e120": true, "e131": true, "e150c": true, "e150d": true, "e210": true,
	"e211": true, "e220": true, "e221": true, "e222": true, "e223": true,
	"e224": true, "e228": true, "e338": true, "e339": true, "e407": true,
	"e450": true, "e451": true, "e452": true,
}

// Regex para porcentajes QUID (ej: 25%, 55,5 %, 0.9%)
var percentRegex = regexp.MustCompile(`(?i)(\d+([.,]\d+)?)\s*%`)

// Regex para códigos de aditivos E-xxx
var eNumberRegex = regexp.MustCompile(`(?i)\bE-?\s*(\d{3,4}[a-z]?)\b`)

// Frases que indican inicio de trazas o información no cuantitativa a descartar
var traceStarters = []string{
	"puede contener trazas de",
	"puede contener",
	"posibles trazas de",
	"trazas de",
	"fabricado en una instalacion que",
	"elaborado en una fabrica que",
	"envasado en una atmosfera",
	"conservacion:",
	"conservar en",
	"almacenar en",
	"modo de empleo",
	"modo de preparacion",
	"sugerencia de presentacion",
}

// Términos de azúcares añadidos y jarabes
var addedSugarTerms = []string{
	"azucar", "jarabe", "glucosa", "fructosa", "dextrosa", "sacarosa",
	"maltodextrina", "sirope", "melaza",
}

// Términos de aceites y grasas refinadas de baja calidad
var refinedFatTerms = []string{
	"palma", "palmiste", "hidrogenada", "hidrogenado",
}

// LabelParser analiza el etiquetado conforme a la normativa de la UE
type LabelParser struct {
	matcher *matcher.RecipeMatcher
}

// NewLabelParser crea un nuevo parser normativo
func NewLabelParser(foods []model.PrimaryFood) *LabelParser {
	return &LabelParser{
		matcher: matcher.NewMatcher(foods),
	}
}

// SplitIngredients separa la lista de ingredientes respetando paréntesis anidados y descartando trazas
func SplitIngredients(text string) []string {
	cleanText := strings.TrimSpace(text)
	if cleanText == "" {
		return nil
	}

	// 1. Descartar trazas y leyendas posteriores
	lower := strings.ToLower(cleanText)
	earliestTraceIdx := -1
	for _, starter := range traceStarters {
		if idx := strings.Index(lower, starter); idx >= 0 {
			if earliestTraceIdx == -1 || idx < earliestTraceIdx {
				earliestTraceIdx = idx
			}
		}
	}
	if earliestTraceIdx >= 0 {
		cleanText = strings.TrimSpace(cleanText[:earliestTraceIdx])
	}

	// 2. Tokenizar respetando paréntesis () y corchetes []
	runes := []rune(cleanText)
	var tokens []string
	var current strings.Builder
	depth := 0

	for i := 0; i < len(runes); i++ {
		r := runes[i]

		// Coma decimal entre números en español (ej: "0,9%", "55,5%") no es separador de ingrediente
		if r == ',' && i > 0 && i < len(runes)-1 && unicode.IsDigit(runes[i-1]) && unicode.IsDigit(runes[i+1]) {
			current.WriteRune(r)
			continue
		}

		switch r {
		case '(', '[', '{':
			depth++
			current.WriteRune(r)
		case ')', ']', '}':
			if depth > 0 {
				depth--
			}
			current.WriteRune(r)
		case ',', ';':
			if depth == 0 {
				token := strings.TrimSpace(current.String())
				if token != "" {
					tokens = append(tokens, token)
				}
				current.Reset()
			} else {
				current.WriteRune(r)
			}
		case '.':
			// Punto solo corta si no está en un número ni dentro de paréntesis
			if depth == 0 && current.Len() > 0 {
				token := strings.TrimSpace(current.String())
				if token != "" && !isNumericEnding(token) {
					tokens = append(tokens, token)
					current.Reset()
					continue
				}
			}
			current.WriteRune(r)
		default:
			current.WriteRune(r)
		}
	}

	if current.Len() > 0 {
		token := strings.TrimSpace(current.String())
		if token != "" {
			tokens = append(tokens, token)
		}
	}

	return tokens
}

func isNumericEnding(s string) bool {
	if len(s) == 0 {
		return false
	}
	last := rune(s[len(s)-1])
	return unicode.IsDigit(last)
}

// DeconstructIngredients deconstruye los ingredientes de un producto comercial
func (lp *LabelParser) DeconstructIngredients(prod model.CommercialProduct) model.LabelDeconstruction {
	tokens := SplitIngredients(prod.IngredientsText)

	deconstruction := model.LabelDeconstruction{
		ProductCode: prod.Code,
		ProductName: prod.ProductName,
		Brands:      prod.Brands,
	}

	seenBases := make(map[string]bool)
	var highRisks []string
	additivesCount := 0
	declaredNoblePct := 0.0
	hasAddedSugars := false
	hasRefinedOils := false

	for i, token := range tokens {
		raw := cleanIngredientToken(token)
		if raw == "" {
			continue
		}

		pos := i + 1
		norm := matcher.Normalize(raw)

		// A. Extraer porcentaje QUID si existe
		var pctVal *float64
		if match := percentRegex.FindStringSubmatch(token); len(match) > 1 {
			s := strings.ReplaceAll(match[1], ",", ".")
			if f, err := strconv.ParseFloat(s, 64); err == nil && f >= 0 && f <= 100 {
				pctVal = &f
			}
		}

		// B. Detectar si es aditivo (E-number o término funcional)
		isAdditive := false
		additiveCode := ""
		additiveRisk := ""

		if match := eNumberRegex.FindStringSubmatch(token); len(match) > 1 {
			isAdditive = true
			codeNorm := "e" + strings.ToLower(match[1])
			additiveCode = "E-" + strings.ToUpper(match[1])

			if highRiskAdditives[codeNorm] {
				additiveRisk = "alto"
				highRisks = append(highRisks, additiveCode)
			} else if mediumRiskAdditives[codeNorm] {
				additiveRisk = "medio"
			} else {
				additiveRisk = "bajo"
			}
			additivesCount++
		} else if isAdditiveCategory(norm) {
			isAdditive = true
			additivesCount++
			additiveRisk = "bajo"
		}

		// C. Detectar azúcares añadidos y grasas refinadas
		for _, s := range addedSugarTerms {
			if strings.Contains(norm, s) {
				hasAddedSugars = true
				break
			}
		}
		for _, f := range refinedFatTerms {
			if strings.Contains(norm, f) {
				hasRefinedOils = true
				break
			}
		}

		// D. Identificar alimento primario noble si no es aditivo
		primaryID := ""
		if !isAdditive {
			cleanName := cleanIngredientNameForMatching(norm)
			if lp.matcher != nil {
				// Buscar base primaria
				pID := lp.findPrimaryBase(cleanName)
				if pID != "" {
					primaryID = pID
					if deconstruction.MainBase == "" {
						deconstruction.MainBase = pID
					}
					if !seenBases[pID] && len(deconstruction.TopBases) < 3 {
						seenBases[pID] = true
						deconstruction.TopBases = append(deconstruction.TopBases, pID)
					}
					if pctVal != nil {
						declaredNoblePct += *pctVal
					}
				}
			}
		}

		deconstruction.Ingredients = append(deconstruction.Ingredients, model.DeconstructedIngredient{
			Position:      pos,
			RawText:       token,
			CleanName:     norm,
			Percentage:    pctVal,
			PrimaryFoodID: primaryID,
			IsAdditive:    isAdditive,
			AdditiveCode:  additiveCode,
			AdditiveRisk:  additiveRisk,
		})
	}

	deconstruction.AdditivesCount = additivesCount
	deconstruction.HighRiskAdditives = highRisks
	deconstruction.DeclaredNoblePercent = declaredNoblePct
	deconstruction.HasAddedSugars = hasAddedSugars
	deconstruction.HasRefinedOils = hasRefinedOils

	// E. Métrica de pureza estimada (0.0 a 100.0)
	purity := calculatePurityScore(prod, deconstruction)
	deconstruction.EstimatedPurityScore = purity

	return deconstruction
}

func (lp *LabelParser) findPrimaryBase(norm string) string {
	// 1. Probar nombre completo limpio
	words := strings.Fields(norm)
	if len(words) == 0 {
		return ""
	}

	// Probar si alguna palabra o subfrase es un alimento primario (ej: "tomate", "calabacin", "avena")
	for _, w := range words {
		if len(w) >= 4 {
			// Usar matcher
			for alias, id := range lp.matcher.GetAliasMap() {
				if alias == w || (len(alias) >= 4 && alias == norm) {
					return id
				}
			}
		}
	}
	return ""
}

func calculatePurityScore(prod model.CommercialProduct, d model.LabelDeconstruction) float64 {
	// Base según Nova Group
	score := 70.0
	switch prod.NovaGroup {
	case "1":
		score = 95.0
	case "2":
		score = 85.0
	case "3":
		score = 70.0
	case "4":
		score = 40.0
	}

	// Bonus si tiene base primaria identificada en 1º puesto
	if d.MainBase != "" {
		score += 5.0
	}

	// Bonus si tiene alto porcentaje de ingrediente noble declarado
	if d.DeclaredNoblePercent >= 70.0 {
		score += 10.0
	} else if d.DeclaredNoblePercent >= 50.0 {
		score += 5.0
	}

	// Penalizaciones
	if d.HasAddedSugars {
		score -= 10.0
	}
	if d.HasRefinedOils {
		score -= 10.0
	}
	for _, _ = range d.HighRiskAdditives {
		score -= 12.0
	}
	if d.AdditivesCount > 3 {
		score -= float64(d.AdditivesCount-3) * 2.0
	}

	if score > 100.0 {
		score = 100.0
	}
	if score < 0.0 {
		score = 0.0
	}
	return score
}

func cleanIngredientToken(token string) string {
	s := strings.TrimSpace(token)
	// Quitar guiones iniciales o viñetas
	s = strings.TrimPrefix(s, "-")
	s = strings.TrimPrefix(s, "*")
	s = strings.TrimPrefix(s, "•")
	// Quitar marcas de subrayado de alérgenos (_almendras_ -> almendras)
	s = strings.ReplaceAll(s, "_", "")
	return strings.TrimSpace(s)
}

func cleanIngredientNameForMatching(norm string) string {
	// Quitar porcentajes si quedaron
	norm = percentRegex.ReplaceAllString(norm, "")
	// Quitar notas de corte o presentación
	norm = strings.ReplaceAll(norm, "desgrasado", "")
	norm = strings.ReplaceAll(norm, "concentrado", "")
	norm = strings.ReplaceAll(norm, "triturado", "")
	norm = strings.ReplaceAll(norm, "polvo", "")
	norm = strings.ReplaceAll(norm, "entero", "")
	norm = strings.ReplaceAll(norm, "entera", "")
	norm = strings.ReplaceAll(norm, "enteras", "")
	norm = strings.ReplaceAll(norm, "molido", "")
	norm = strings.ReplaceAll(norm, "molida", "")
	return strings.TrimSpace(norm)
}

func isAdditiveCategory(norm string) bool {
	categories := []string{
		"emulgente", "emulsionante", "conservador", "conservante", "antioxidante",
		"acidulante", "espesante", "estabilizador", "estabilizante", "colorante",
		"potenciador del sabor", "gasificante", "edulcorante", "humectante",
		"antiaglomerante", "gelificante",
	}
	for _, c := range categories {
		if strings.Contains(norm, c) {
			return true
		}
	}
	return false
}
