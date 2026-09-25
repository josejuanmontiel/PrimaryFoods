package importer

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"
	"unicode"

	"github.com/josejuanmontiel/primaryfoods/internal/model"
)

// QwenRecipe representa la estructura generada por el pipeline de extracción Qwen-VL / Schema.org
type QwenRecipe struct {
	Name               string      `json:"name"`
	Description        string      `json:"description"`
	Servings           int         `json:"servings"`
	Yield              int         `json:"yield"`
	RecipeYield        interface{} `json:"recipeYield"`
	PrepTimeMinutes    *int        `json:"prep_time_minutes"`
	CookTimeMinutes    *int        `json:"cook_time_minutes"`
	TotalTime          string      `json:"totalTime"`
	PrepTime           string      `json:"prepTime"`
	CookTime           string      `json:"cookTime"`
	Tags               []string    `json:"tags"`
	Keywords           interface{} `json:"keywords"`
	Image              interface{} `json:"image"`
	RecipeIngredient   []string    `json:"recipeIngredient"`
	RecipeInstructions interface{} `json:"recipeInstructions"`
	Ingredients        []struct {
		Name         string      `json:"name"`
		Amount       interface{} `json:"amount"` // puede venir número o string
		Unit         string      `json:"unit"`
		OriginalText string      `json:"originalText"`
	} `json:"ingredients"`
	Instructions interface{} `json:"instructions"` // puede ser []string o string
	Source       string      `json:"source"`
}

// LoadRecipesFromJSONFile carga recetas desde un archivo JSON (array de recetas o receta única)
func LoadRecipesFromJSONFile(filePath string) ([]model.Recipe, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("error abriendo archivo de recetas: %w", err)
	}
	defer f.Close()

	return LoadRecipesFromJSON(f)
}

// LoadRecipesFromJSON lee recetas desde un io.Reader
func LoadRecipesFromJSON(r io.Reader) ([]model.Recipe, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}

	// Probar si es array
	var rawList []QwenRecipe
	if err := json.Unmarshal(data, &rawList); err != nil {
		// Probar si es un solo objeto
		var single QwenRecipe
		if err2 := json.Unmarshal(data, &single); err2 != nil {
			return nil, fmt.Errorf("error parseando JSON de recetas: %v", err)
		}
		rawList = []QwenRecipe{single}
	}

	var recipes []model.Recipe
	for _, raw := range rawList {
		if strings.TrimSpace(raw.Name) == "" {
			continue
		}

		slug := Slugify(raw.Name)
		servings := raw.Servings
		if servings <= 0 {
			servings = raw.Yield
		}
		if servings <= 0 && raw.RecipeYield != nil {
			switch v := raw.RecipeYield.(type) {
			case float64:
				servings = int(v)
			case int:
				servings = v
			case string:
				var parsed int
				if n, _ := fmt.Sscanf(v, "%d", &parsed); n > 0 {
					servings = parsed
				}
			}
		}
		if servings <= 0 {
			servings = 2
		}

		var ingredients []model.RecipeIngredient
		for _, ing := range raw.Ingredients {
			var amt float64
			switch v := ing.Amount.(type) {
			case float64:
				amt = v
			case int:
				amt = float64(v)
			case string:
				var parsed float64
				if n, _ := fmt.Sscanf(strings.ReplaceAll(v, ",", "."), "%f", &parsed); n > 0 {
					amt = parsed
				}
			}

			orig := ing.OriginalText
			if orig == "" {
				orig = ing.Name
			}

			ingredients = append(ingredients, model.RecipeIngredient{
				Name:         strings.TrimSpace(ing.Name),
				Amount:       amt,
				Unit:         strings.TrimSpace(ing.Unit),
				OriginalText: orig,
			})
		}

		// Fallback a RecipeIngredient de Schema.org si ingredients venía vacío
		if len(ingredients) == 0 && len(raw.RecipeIngredient) > 0 {
			for _, line := range raw.RecipeIngredient {
				line = strings.TrimSpace(line)
				if line == "" {
					continue
				}
				ingredients = append(ingredients, parseRawIngredientLine(line))
			}
		}

		var instructions []string
		extractInstructions := func(val interface{}) {
			switch inst := val.(type) {
			case []interface{}:
				for _, step := range inst {
					switch s := step.(type) {
					case string:
						if strings.TrimSpace(s) != "" {
							instructions = append(instructions, strings.TrimSpace(s))
						}
					case map[string]interface{}:
						// Schema.org HowToStep
						if text, ok := s["text"].(string); ok && strings.TrimSpace(text) != "" {
							instructions = append(instructions, strings.TrimSpace(text))
						}
					}
				}
			case []string:
				for _, s := range inst {
					if strings.TrimSpace(s) != "" {
						instructions = append(instructions, strings.TrimSpace(s))
					}
				}
			case string:
				lines := strings.Split(inst, "\n")
				for _, l := range lines {
					if strings.TrimSpace(l) != "" {
						instructions = append(instructions, strings.TrimSpace(l))
					}
				}
			}
		}

		extractInstructions(raw.Instructions)
		if len(instructions) == 0 && raw.RecipeInstructions != nil {
			extractInstructions(raw.RecipeInstructions)
		}

		// Extraer imagen
		var imageStr string
		switch img := raw.Image.(type) {
		case string:
			imageStr = strings.TrimSpace(img)
		case []interface{}:
			if len(img) > 0 {
				if s, ok := img[0].(string); ok {
					imageStr = strings.TrimSpace(s)
				}
			}
		case map[string]interface{}:
			if u, ok := img["url"].(string); ok {
				imageStr = strings.TrimSpace(u)
			}
		}

		// Tags / Keywords
		tags := raw.Tags
		if len(tags) == 0 && raw.Keywords != nil {
			switch kw := raw.Keywords.(type) {
			case string:
				parts := strings.Split(kw, ",")
				for _, p := range parts {
					p = strings.TrimSpace(p)
					if p != "" {
						tags = append(tags, p)
					}
				}
			case []interface{}:
				for _, p := range kw {
					if s, ok := p.(string); ok && strings.TrimSpace(s) != "" {
						tags = append(tags, strings.TrimSpace(s))
					}
				}
			}
		}

		totalTime := raw.TotalTime
		if totalTime == "" && raw.CookTime != "" {
			totalTime = raw.CookTime
		}

		source := raw.Source
		if source == "" {
			source = "qwen_ocr"
		}

		recipes = append(recipes, model.Recipe{
			ID:           "recipe:" + slug,
			Slug:         slug,
			Name:         strings.TrimSpace(raw.Name),
			Description:  strings.TrimSpace(raw.Description),
			Yield:        servings,
			Ingredients:  ingredients,
			Instructions: instructions,
			Tags:         tags,
			TotalTime:    totalTime,
			Image:        imageStr,
			Source:       source,
		})
	}

	return recipes, nil
}

func parseRawIngredientLine(text string) model.RecipeIngredient {
	text = strings.TrimSpace(text)
	// Intentar extraer cantidad numérica inicial (ej: "200 g de tomate", "2 cebollas")
	var amt float64 = 1.0
	var unit string = "unidad"
	var name string = text

	var parsedAmt float64
	var parsedUnit, parsedName string
	n, _ := fmt.Sscanf(text, "%f %s %s", &parsedAmt, &parsedUnit, &parsedName)
	if n >= 2 && parsedAmt > 0 {
		amt = parsedAmt
		uLower := strings.ToLower(parsedUnit)
		if uLower == "g" || uLower == "gr" || uLower == "gramos" {
			unit = "g"
			name = strings.TrimPrefix(text[len(fmt.Sprintf("%v %s", parsedAmt, parsedUnit)):], " ")
		} else if uLower == "ml" || uLower == "mililitros" || uLower == "cc" {
			unit = "ml"
			name = strings.TrimPrefix(text[len(fmt.Sprintf("%v %s", parsedAmt, parsedUnit)):], " ")
		} else {
			name = text[len(fmt.Sprintf("%v", parsedAmt)): ]
		}
		name = strings.TrimPrefix(strings.TrimSpace(name), "de ")
	}

	return model.RecipeIngredient{
		Name:         strings.TrimSpace(name),
		Amount:       amt,
		Unit:         unit,
		OriginalText: text,
	}
}

var nonAlphanumericRegex = regexp.MustCompile(`[^a-z0-9]+`)

// Slugify convierte un título en un slug URL-friendly
func Slugify(text string) string {
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
		case 'ñ':
			b.WriteRune('n')
		case 'ç':
			b.WriteRune('c')
		default:
			if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '-' || r == ' ' {
				b.WriteRune(r)
			} else {
				b.WriteRune(' ')
			}
		}
	}

	slug := nonAlphanumericRegex.ReplaceAllString(b.String(), "-")
	slug = strings.Trim(slug, "-")
	if slug == "" {
		slug = "receta"
	}
	return slug
}
