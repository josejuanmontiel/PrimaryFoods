package mealie

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/josejuanmontiel/primaryfoods/internal/model"
)

// Client se comunica con la API REST de Mealie
type Client struct {
	BaseURL    string
	Token      string
	HTTPClient *http.Client
}

// NewClient crea una nueva instancia de cliente Mealie
func NewClient(baseURL, token string) *Client {
	baseURL = strings.TrimRight(baseURL, "/")
	if baseURL == "" {
		baseURL = "http://localhost:9925"
	}
	return &Client{
		BaseURL: baseURL,
		Token:   strings.TrimSpace(token),
		HTTPClient: &http.Client{
			Timeout: 15 * time.Second,
		},
	}
}

// MealieIngredientRaw representa la estructura interna de ingrediente en la API de Mealie
type MealieIngredientRaw struct {
	Title        string      `json:"title"`
	Note         string      `json:"note"`
	Display      string      `json:"display"`
	Quantity     interface{} `json:"quantity"`
	Unit         interface{} `json:"unit"` // Puede ser string o {"name": "..."}
	Food         interface{} `json:"food"` // Puede ser string o {"name": "..."}
	OriginalText string      `json:"originalText"`
}

// MealieRecipeDetail representa la respuesta detallada de una receta en Mealie
type MealieRecipeDetail struct {
	ID                 string                `json:"id"`
	Slug               string                `json:"slug"`
	Name               string                `json:"name"`
	Description        string                `json:"description"`
	RecipeYield        interface{}           `json:"recipeYield"`
	TotalTime          string                `json:"totalTime"`
	RecipeIngredient   []MealieIngredientRaw `json:"recipeIngredient"`
	RecipeInstructions []interface{}         `json:"recipeInstructions"`
	Tags               []interface{}         `json:"tags"`
	Image              string                `json:"image"`
}

// MealieListResponse representa la respuesta paginada de GET /api/recipes
type MealieListResponse struct {
	Page       int                  `json:"page"`
	PerPage    int                  `json:"per_page"`
	Total      int                  `json:"total"`
	TotalPages int                  `json:"total_pages"`
	Items      []MealieRecipeDetail `json:"items"`
}

// TestConnection comprueba la conectividad con Mealie
func (c *Client) TestConnection(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, "GET", c.BaseURL+"/api/app/about", nil)
	if err != nil {
		return err
	}
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		// Intentar endpoint alternativo
		reqAlt, _ := http.NewRequestWithContext(ctx, "GET", c.BaseURL+"/api/recipes?perPage=1", nil)
		if c.Token != "" {
			reqAlt.Header.Set("Authorization", "Bearer "+c.Token)
		}
		respAlt, errAlt := c.HTTPClient.Do(reqAlt)
		if errAlt != nil {
			return fmt.Errorf("no se pudo conectar con Mealie en %s: %w", c.BaseURL, err)
		}
		defer respAlt.Body.Close()
		if respAlt.StatusCode >= 400 && respAlt.StatusCode != http.StatusUnauthorized {
			return fmt.Errorf("error del servidor Mealie: status %d", respAlt.StatusCode)
		}
		return nil
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 && resp.StatusCode != http.StatusUnauthorized {
		return fmt.Errorf("error conectando con Mealie: HTTP %d", resp.StatusCode)
	}
	return nil
}

// GetRecipes obtiene todas las recetas desde Mealie y las normaliza a model.Recipe
func (c *Client) GetRecipes(ctx context.Context) ([]model.Recipe, error) {
	url := fmt.Sprintf("%s/api/recipes?page=1&perPage=250", c.BaseURL)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error llamando a Mealie /api/recipes: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("Mealie devolvió status %d: %s", resp.StatusCode, string(body))
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var listResp MealieListResponse
	var rawItems []MealieRecipeDetail

	if err := json.Unmarshal(bodyBytes, &listResp); err == nil && len(listResp.Items) > 0 {
		rawItems = listResp.Items
	} else if err := json.Unmarshal(bodyBytes, &rawItems); err != nil {
		return nil, fmt.Errorf("formato no reconocido de respuesta de Mealie: %w", err)
	}

	var normalized []model.Recipe
	for _, raw := range rawItems {
		// Si faltan los ingredientes en la lista resumida, intentamos obtener el detalle completo
		if len(raw.RecipeIngredient) == 0 && raw.Slug != "" {
			detail, err := c.GetRecipeDetail(ctx, raw.Slug)
			if err == nil && detail != nil {
				raw = *detail
			}
		}
		normalized = append(normalized, ConvertMealieToModel(raw))
	}

	return normalized, nil
}

// GetRecipeDetail obtiene el detalle completo de una receta por su slug
func (c *Client) GetRecipeDetail(ctx context.Context, slug string) (*MealieRecipeDetail, error) {
	url := fmt.Sprintf("%s/api/recipes/%s", c.BaseURL, slug)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d al obtener receta %s", resp.StatusCode, slug)
	}

	var detail MealieRecipeDetail
	if err := json.NewDecoder(resp.Body).Decode(&detail); err != nil {
		return nil, err
	}
	return &detail, nil
}

// ParseRecipesFromJSONFile lee recetas desde un archivo JSON (útil para pruebas offline y recetas generadas)
func ParseRecipesFromJSONFile(filePath string) ([]model.Recipe, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}
	return ParseRecipesFromJSON(data)
}

// ParseRecipesFromJSON parsea recetas desde bytes JSON
func ParseRecipesFromJSON(data []byte) ([]model.Recipe, error) {
	// Intentar primero formato de recetas generadas: [{"name": "...", "ingredients": [{"name": "..."}]}]
	type simpleIngredient struct {
		Name   string  `json:"name"`
		Amount float64 `json:"amount"`
		Unit   string  `json:"unit"`
	}
	type simpleRecipe struct {
		Name         string             `json:"name"`
		Description  string             `json:"description"`
		Servings     int                `json:"servings"`
		Instructions string             `json:"instructions"`
		Tags         []string           `json:"tags"`
		Ingredients  []simpleIngredient `json:"ingredients"`
	}

	var simpleList []simpleRecipe
	if err := json.Unmarshal(data, &simpleList); err == nil && len(simpleList) > 0 && simpleList[0].Name != "" {
		var result []model.Recipe
		for i, s := range simpleList {
			var ings []model.RecipeIngredient
			for _, ing := range s.Ingredients {
				ings = append(ings, model.RecipeIngredient{
					Name:   ing.Name,
					Amount: ing.Amount,
					Unit:   ing.Unit,
				})
			}
			var inst []string
			if s.Instructions != "" {
				inst = strings.Split(s.Instructions, ". ")
			}
			slug := strings.ToLower(strings.ReplaceAll(s.Name, " ", "-"))
			result = append(result, model.Recipe{
				ID:           fmt.Sprintf("recipe-%d", i+1),
				Slug:         slug,
				Name:         s.Name,
				Description:  s.Description,
				Yield:        s.Servings,
				Ingredients:  ings,
				Instructions: inst,
				Tags:         s.Tags,
				Source:       "json_file",
			})
		}
		return result, nil
	}

	// Si no, intentar formato directo model.Recipe
	var directList []model.Recipe
	if err := json.Unmarshal(data, &directList); err == nil && len(directList) > 0 {
		return directList, nil
	}

	return nil, fmt.Errorf("no se pudo parsear el archivo JSON como recetas válidas")
}

// ConvertMealieToModel transforma una receta de Mealie a model.Recipe
func ConvertMealieToModel(m MealieRecipeDetail) model.Recipe {
	var ingredients []model.RecipeIngredient
	for _, ing := range m.RecipeIngredient {
		ingredients = append(ingredients, ParseMealieIngredient(ing))
	}

	var instructions []string
	for _, inst := range m.RecipeInstructions {
		switch v := inst.(type) {
		case string:
			instructions = append(instructions, v)
		case map[string]interface{}:
			if text, ok := v["text"].(string); ok && text != "" {
				instructions = append(instructions, text)
			}
		}
	}

	var tags []string
	for _, t := range m.Tags {
		switch v := t.(type) {
		case string:
			tags = append(tags, v)
		case map[string]interface{}:
			if name, ok := v["name"].(string); ok && name != "" {
				tags = append(tags, name)
			}
		}
	}

	yield := 2
	switch v := m.RecipeYield.(type) {
	case float64:
		yield = int(v)
	case string:
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			yield = n
		}
	}

	imgURL := ""
	if m.Image != "" {
		if strings.HasPrefix(m.Image, "http://") || strings.HasPrefix(m.Image, "https://") {
			imgURL = m.Image
		} else {
			imgURL = fmt.Sprintf("/api/media/recipes/%s/images/original.webp", m.ID)
		}
	}

	return model.Recipe{
		ID:           m.ID,
		Slug:         m.Slug,
		Name:         m.Name,
		Description:  m.Description,
		Yield:        yield,
		Ingredients:  ingredients,
		Instructions: instructions,
		Tags:         tags,
		TotalTime:    m.TotalTime,
		Image:        imgURL,
		Source:       "mealie",
	}
}

var ingredientRegex = regexp.MustCompile(`^([\d\.\,\/]+)\s*([a-zA-ZáéíóúÁÉÍÓÚñÑ]+)?\s*(.*)$`)

// ParseMealieIngredient extrae name, amount y unit de un ingrediente de Mealie
func ParseMealieIngredient(raw MealieIngredientRaw) model.RecipeIngredient {
	// Si food está estructurado como objeto {"name": "..."}
	var foodName string
	if foodMap, ok := raw.Food.(map[string]interface{}); ok {
		if fn, ok := foodMap["name"].(string); ok {
			foodName = fn
		}
	} else if fn, ok := raw.Food.(string); ok {
		foodName = fn
	}

	var unitName string
	if unitMap, ok := raw.Unit.(map[string]interface{}); ok {
		if un, ok := unitMap["name"].(string); ok {
			unitName = un
		}
	} else if un, ok := raw.Unit.(string); ok {
		unitName = un
	}

	qty := 1.0
	switch v := raw.Quantity.(type) {
	case float64:
		qty = v
	case string:
		if f, err := strconv.ParseFloat(strings.ReplaceAll(v, ",", "."), 64); err == nil {
			qty = f
		}
	}

	if foodName != "" {
		if unitName == "" {
			unitName = "unidad"
		}
		return model.RecipeIngredient{
			Name:         foodName,
			Amount:       qty,
			Unit:         unitName,
			OriginalText: raw.Display,
		}
	}

	// Si no viene estructurado, parsear note u originalText (suelen ser más limpios que display)
	lineText := raw.Note
	if lineText == "" {
		lineText = raw.OriginalText
	}
	if lineText == "" {
		lineText = raw.Display
	}
	if lineText == "" {
		lineText = raw.Title
	}

	lineText = strings.TrimSpace(lineText)
	matches := ingredientRegex.FindStringSubmatch(lineText)
	if len(matches) == 4 {
		amtStr := matches[1]
		unitStr := strings.ToLower(matches[2])
		rest := matches[3]

		var parsedAmount float64 = 1.0
		if strings.Contains(amtStr, "/") {
			parts := strings.Split(amtStr, "/")
			if len(parts) == 2 {
				n, _ := strconv.ParseFloat(parts[0], 64)
				d, _ := strconv.ParseFloat(parts[1], 64)
				if d > 0 {
					parsedAmount = n / d
				}
			}
		} else {
			f, _ := strconv.ParseFloat(strings.ReplaceAll(amtStr, ",", "."), 64)
			if f > 0 {
				parsedAmount = f
			}
		}

		unit := "g"
		switch unitStr {
		case "g", "gr", "gramos", "gram":
			unit = "g"
		case "ml", "mililitros", "cc":
			unit = "ml"
		case "ud", "uds", "unidad", "unidades", "pieza", "piezas":
			unit = "unidad"
		default:
			if unitStr != "" {
				rest = unitStr + " " + rest
			}
			unit = "unidad"
		}

		name := strings.TrimSpace(rest)
		if name == "" {
			name = lineText
		}

		return model.RecipeIngredient{
			Name:         name,
			Amount:       parsedAmount,
			Unit:         unit,
			OriginalText: lineText,
		}
	}

	return model.RecipeIngredient{
		Name:         lineText,
		Amount:       1,
		Unit:         "unidad",
		OriginalText: lineText,
	}
}

func slugify(text string) string {
	text = strings.ToLower(strings.TrimSpace(text))
	text = strings.ReplaceAll(text, "á", "a")
	text = strings.ReplaceAll(text, "é", "e")
	text = strings.ReplaceAll(text, "í", "i")
	text = strings.ReplaceAll(text, "ó", "o")
	text = strings.ReplaceAll(text, "ú", "u")
	text = strings.ReplaceAll(text, "ñ", "n")
	reg := regexp.MustCompile(`[^a-z0-9]+`)
	slug := reg.ReplaceAllString(text, "-")
	return strings.Trim(slug, "-")
}

// CreateOrUpdateRecipe exporta o sube una receta a Mealie usando la API oficial (POST + PATCH)
func (c *Client) CreateOrUpdateRecipe(ctx context.Context, recipe model.Recipe) (string, error) {
	if recipe.Name == "" {
		return "", fmt.Errorf("nombre de receta requerido")
	}

	// 1. Crear la receta básica vía POST /api/recipes para obtener el slug
	createURL := fmt.Sprintf("%s/api/recipes", c.BaseURL)
	createPayload := map[string]string{"name": recipe.Name}
	createBytes, _ := json.Marshal(createPayload)

	req, err := http.NewRequestWithContext(ctx, "POST", createURL, strings.NewReader(string(createBytes)))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("error conectando con Mealie: %w", err)
	}
	defer resp.Body.Close()

	var slug string
	if resp.StatusCode == http.StatusCreated || resp.StatusCode == http.StatusOK {
		var res interface{}
		if err := json.NewDecoder(resp.Body).Decode(&res); err == nil {
			switch v := res.(type) {
			case string:
				slug = v
			case map[string]interface{}:
				if s, ok := v["slug"].(string); ok && s != "" {
					slug = s
				} else if s, ok := v["name"].(string); ok {
					slug = s
				}
			}
		}
	} else {
		if recipe.Slug != "" {
			slug = recipe.Slug
		} else {
			slug = slugify(recipe.Name)
		}
	}

	if slug == "" {
		if recipe.Slug != "" {
			slug = recipe.Slug
		} else {
			slug = slugify(recipe.Name)
		}
	}

	// 2. Obtener la receta base de Mealie para preservar identificadores internos
	_, err = c.GetRecipeDetail(ctx, slug)
	if err != nil {
		if recipe.Slug != "" && recipe.Slug != slug {
			slug = recipe.Slug
			_, err = c.GetRecipeDetail(ctx, slug)
		}
		if err != nil {
			return "", fmt.Errorf("error obteniendo detalle de receta base '%s' en Mealie: %w", slug, err)
		}
	}

	// 3. Convertir ingredientes a la estructura esperada por Mealie
	var mealieIngredients []map[string]interface{}
	for _, ing := range recipe.Ingredients {
		noteText := ing.OriginalText
		if noteText == "" {
			if ing.Amount > 0 && ing.Unit != "" {
				noteText = fmt.Sprintf("%.2g %s %s", ing.Amount, ing.Unit, ing.Name)
			} else if ing.Amount > 0 {
				noteText = fmt.Sprintf("%.2g %s", ing.Amount, ing.Name)
			} else {
				noteText = ing.Name
			}
		}
		noteText = strings.TrimSpace(noteText)

		mealieIngredients = append(mealieIngredients, map[string]interface{}{
			"quantity":     ing.Amount,
			"unit":         nil,
			"food":         nil,
			"note":         noteText,
			"display":      noteText,
			"title":        nil,
			"originalText": noteText,
			"referenceId":  nil,
		})
	}

	// 4. Convertir instrucciones a formato steps
	var mealieInstructions []map[string]interface{}
	for _, step := range recipe.Instructions {
		step = strings.TrimSpace(step)
		if step == "" {
			continue
		}
		mealieInstructions = append(mealieInstructions, map[string]interface{}{
			"title":                "",
			"summary":              "",
			"text":                 step,
			"ingredientReferences": []interface{}{},
		})
	}

	// 5. Preparar PATCH
	servings := recipe.Yield
	if servings <= 0 {
		servings = 2
	}

	patchPayload := map[string]interface{}{
		"description":        recipe.Description,
		"recipeServings":     servings,
		"recipeIngredient":   mealieIngredients,
		"recipeInstructions": mealieInstructions,
	}

	if len(recipe.Tags) > 0 {
		var tagObjs []map[string]string
		for _, t := range recipe.Tags {
			t = strings.TrimSpace(t)
			if t == "" {
				continue
			}
			tagObjs = append(tagObjs, map[string]string{
				"name": t,
				"slug": slugify(t),
			})
		}
		patchPayload["tags"] = tagObjs
	}
	if recipe.TotalTime != "" {
		patchPayload["totalTime"] = recipe.TotalTime
	}

	patchURL := fmt.Sprintf("%s/api/recipes/%s", c.BaseURL, slug)
	patchBytes, _ := json.Marshal(patchPayload)

	patchReq, err := http.NewRequestWithContext(ctx, "PATCH", patchURL, strings.NewReader(string(patchBytes)))
	if err != nil {
		return "", err
	}
	patchReq.Header.Set("Content-Type", "application/json")
	if c.Token != "" {
		patchReq.Header.Set("Authorization", "Bearer "+c.Token)
	}

	patchResp, err := c.HTTPClient.Do(patchReq)
	if err != nil {
		return "", fmt.Errorf("error enviando actualización a Mealie: %w", err)
	}
	defer patchResp.Body.Close()

	if patchResp.StatusCode != http.StatusOK && patchResp.StatusCode != http.StatusNoContent {
		b, _ := io.ReadAll(patchResp.Body)
		return "", fmt.Errorf("HTTP %d al actualizar receta '%s' en Mealie: %s", patchResp.StatusCode, slug, string(b))
	}

	// 6. Subir imagen a Mealie si la receta tiene imagen asociada (base64, URL o archivo local)
	if recipe.Image != "" {
		if err := c.UploadRecipeImageFromString(ctx, slug, recipe.Image); err != nil {
			fmt.Printf("⚠️ No se pudo subir imagen a Mealie para '%s': %v\n", slug, err)
		}
	}

	return slug, nil
}

// UploadRecipeImageFromString sube una imagen a Mealie a partir de un string (data URI base64, URL remota o ruta local)
func (c *Client) UploadRecipeImageFromString(ctx context.Context, slug, imageStr string) error {
	imageStr = strings.TrimSpace(imageStr)
	if imageStr == "" {
		return nil
	}

	var imgBytes []byte
	ext := "jpg"

	if strings.HasPrefix(imageStr, "data:") {
		// Formato data URI: data:image/jpeg;base64,...
		parts := strings.SplitN(imageStr, ",", 2)
		if len(parts) == 2 {
			mimePart := parts[0]
			if strings.Contains(mimePart, "png") {
				ext = "png"
			} else if strings.Contains(mimePart, "webp") {
				ext = "webp"
			}
			b, err := base64.StdEncoding.DecodeString(parts[1])
			if err != nil {
				return fmt.Errorf("error decodificando imagen base64: %w", err)
			}
			imgBytes = b
		}
	} else if strings.HasPrefix(imageStr, "http://") || strings.HasPrefix(imageStr, "https://") {
		// Es URL externa -> Scrapear usando endpoint de Mealie
		return c.ScrapeRecipeImageURL(ctx, slug, imageStr)
	} else if _, err := os.Stat(imageStr); err == nil {
		// Es ruta de archivo en disco
		b, err := os.ReadFile(imageStr)
		if err != nil {
			return err
		}
		imgBytes = b
		if strings.HasSuffix(strings.ToLower(imageStr), ".png") {
			ext = "png"
		} else if strings.HasSuffix(strings.ToLower(imageStr), ".webp") {
			ext = "webp"
		}
	} else {
		// Probar base64 directo sin cabecera data:
		b, err := base64.StdEncoding.DecodeString(imageStr)
		if err == nil && len(b) > 100 {
			imgBytes = b
		}
	}

	if len(imgBytes) == 0 {
		return nil
	}

	return c.UploadRecipeImage(ctx, slug, imgBytes, ext)
}

// UploadRecipeImage sube el archivo binario de la imagen a Mealie vía PUT /api/recipes/{slug}/image
func (c *Client) UploadRecipeImage(ctx context.Context, slug string, imgBytes []byte, ext string) error {
	if ext == "" {
		ext = "jpg"
	}

	targetURL := fmt.Sprintf("%s/api/recipes/%s/image", c.BaseURL, slug)
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	// Campo extension requerido por la API de Mealie
	if err := writer.WriteField("extension", ext); err != nil {
		return err
	}

	// Campo image (archivo binario)
	part, err := writer.CreateFormFile("image", fmt.Sprintf("recipe.%s", ext))
	if err != nil {
		return err
	}
	if _, err := part.Write(imgBytes); err != nil {
		return err
	}
	if err := writer.Close(); err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, "PUT", targetURL, body)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("error enviando imagen a Mealie: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("HTTP %d al subir imagen a Mealie para '%s': %s", resp.StatusCode, slug, string(b))
	}

	return nil
}

// ScrapeRecipeImageURL asocia una URL de imagen a la receta usando POST /api/recipes/{slug}/image de Mealie
func (c *Client) ScrapeRecipeImageURL(ctx context.Context, slug, imageURL string) error {
	targetURL := fmt.Sprintf("%s/api/recipes/%s/image", c.BaseURL, slug)
	payload := map[string]string{"url": imageURL}
	data, _ := json.Marshal(payload)

	req, err := http.NewRequestWithContext(ctx, "POST", targetURL, bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("HTTP %d al asociar URL de imagen en Mealie: %s", resp.StatusCode, string(b))
	}
	return nil
}

