package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/josejuanmontiel/primaryfoods/internal/config"
	"github.com/josejuanmontiel/primaryfoods/internal/importer"
	"github.com/josejuanmontiel/primaryfoods/internal/matcher"
	"github.com/josejuanmontiel/primaryfoods/internal/mealie"
	"github.com/josejuanmontiel/primaryfoods/internal/model"
	"github.com/josejuanmontiel/primaryfoods/internal/planner"
	"github.com/josejuanmontiel/primaryfoods/internal/shelflife"
	"github.com/josejuanmontiel/primaryfoods/internal/storage"
)

// Server expone la API HTTP de PrimaryFoods
type Server struct {
	storage      *storage.Storage
	mux          *http.ServeMux
	cfg          *config.Config
	mealieClient *mealie.Client
	matcher      *matcher.RecipeMatcher
	analyzer     *matcher.RecipeAnalyzer
	labelParser  *importer.LabelParser
	planner      *planner.MenuPlanner
}

// NewServer crea un nuevo servidor HTTP con todas sus rutas registradas
func NewServer(store *storage.Storage, optCfg ...*config.Config) *Server {
	var cfg *config.Config
	if len(optCfg) > 0 && optCfg[0] != nil {
		cfg = optCfg[0]
	} else {
		var err error
		cfg, err = config.LoadConfig("config.json")
		if err != nil {
			cfg = config.DefaultConfig()
		}
	}

	foods, _ := store.ExportAll()
	recMatcher := matcher.NewMatcher(foods)

	s := &Server{
		storage:      store,
		mux:          http.NewServeMux(),
		cfg:          cfg,
		mealieClient: mealie.NewClient(cfg.MealieURL, cfg.MealieToken),
		matcher:      recMatcher,
		analyzer:     matcher.NewRecipeAnalyzer(store, foods),
		labelParser:  importer.NewLabelParser(foods),
		planner:      planner.NewMenuPlanner(recMatcher),
	}
	s.routes()
	return s
}

// Handler devuelve el handler HTTP principal con middleware de CORS y logging
func (s *Server) Handler() http.Handler {
	return s.corsMiddleware(s.mux)
}

func (s *Server) routes() {
	s.mux.HandleFunc("GET /api/health", s.handleHealth)
	s.mux.HandleFunc("GET /api/foods", s.handleListFoods)
	s.mux.HandleFunc("GET /api/foods/search", s.handleSearchFoods)
	s.mux.HandleFunc("GET /api/foods/{id}", s.handleGetFoodByID)
	s.mux.HandleFunc("POST /api/foods", s.handleUpsertFood)
	s.mux.HandleFunc("GET /api/export/json", s.handleExportJSON)

	// Configuración de la aplicación y credenciales
	s.mux.HandleFunc("GET /api/config", s.handleGetConfig)
	s.mux.HandleFunc("POST /api/config", s.handleUpdateConfig)

	// Sincronización Buscatureceta
	s.mux.HandleFunc("POST /api/sync/buscatureceta", s.handleSyncBuscatureceta)
	s.mux.HandleFunc("GET /api/pantry", s.handleGetPantry)
	s.mux.HandleFunc("GET /api/pantry/shelf-life/estimate", s.handleEstimateShelfLife)

	// Estacionalidad y Calendario
	s.mux.HandleFunc("GET /api/seasonal/calendar", s.handleGetSeasonalCalendar)
	s.mux.HandleFunc("POST /api/seasonal/import", s.handleImportSeasonal)

	// Eficiencia Energética de Cocinado
	s.mux.HandleFunc("GET /api/cooking/energy-profiles", s.handleGetCookingEnergyProfiles)
	s.mux.HandleFunc("POST /api/cooking/import", s.handleImportCookingProfiles)

	// Planificación y Generador de Menús (Semanal y Anual)
	s.mux.HandleFunc("GET /api/menu/generate", s.handleGenerateWeeklyMenu)
	s.mux.HandleFunc("POST /api/menu/generate", s.handlePostGenerateWeeklyMenu)
	s.mux.HandleFunc("GET /api/menu/annual", s.handleGenerateAnnualMenu)
	s.mux.HandleFunc("GET /api/menu/annual/export", s.handleExportAnnualMenu)

	// Conexión y sincronización Mealie
	s.mux.HandleFunc("GET /api/mealie/status", s.handleMealieStatus)
	s.mux.HandleFunc("GET /api/mealie/diff", s.handleMealieDiff)
	s.mux.HandleFunc("POST /api/mealie/import", s.handleMealieImport)
	s.mux.HandleFunc("POST /api/mealie/sync", s.handleMealieSync)
	s.mux.HandleFunc("POST /api/mealie/export", s.handleMealieExport)

	// Motor Despensa -> Receta
	s.mux.HandleFunc("GET /api/recipes/matching", s.handleGetRecipesMatching)
	s.mux.HandleFunc("POST /api/recipes/matching", s.handlePostRecipesMatching)

	// Importación de Recetas JSON / Schema.org y Análisis Profundo
	s.mux.HandleFunc("POST /api/recipes/import-json", s.handleImportRecipeJSON)
	s.mux.HandleFunc("GET /api/recipes/analyze", s.handleAnalyzeRecipes)

	// Productos comerciales OpenFoodFacts & Deconstrucción Normativa
	s.mux.HandleFunc("GET /api/products/search", s.handleSearchProducts)
	s.mux.HandleFunc("GET /api/products/by-base", s.handleGetProductsByBase)
	s.mux.HandleFunc("GET /api/products/{code}/deconstruct", s.handleDeconstructProduct)
	s.mux.HandleFunc("POST /api/products/import-off", s.handleImportOFF)
	s.mux.HandleFunc("POST /api/mappings", s.handleSaveMapping)

	// Exportación Enriquecida para Buscatureceta
	s.mux.HandleFunc("GET /api/export/buscatureceta", s.handleExportBuscatureceta)

	// Documentación OpenAPI 3.1
	s.mux.HandleFunc("GET /api/openapi.yaml", s.handleOpenAPIYAML)
	s.mux.HandleFunc("GET /api/openapi.json", s.handleOpenAPIJSON)
	s.mux.HandleFunc("GET /docs", s.handleAPIDocs)

	// Interfaz Web de PrimaryFoods
	s.mux.HandleFunc("GET /", s.handleIndexUI)
}

// Middleware de CORS para permitir consultas directas desde Buscatureceta SPA
func (s *Server) corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func (s *Server) jsonResponse(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if data != nil {
		json.NewEncoder(w).Encode(data)
	}
}

func (s *Server) errorResponse(w http.ResponseWriter, status int, message string) {
	s.jsonResponse(w, status, map[string]string{"error": message})
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	s.jsonResponse(w, http.StatusOK, map[string]interface{}{
		"status":  "ok",
		"service": "PrimaryFoods API",
		"version": "1.0.0",
	})
}

func (s *Server) handleListFoods(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	limit, _ := strconv.Atoi(query.Get("limit"))
	offset, _ := strconv.Atoi(query.Get("offset"))

	filter := model.SearchFilter{
		Category: strings.TrimSpace(query.Get("category")),
		Tag:      strings.TrimSpace(query.Get("tag")),
		Limit:    limit,
		Offset:   offset,
	}

	foods, total, err := s.storage.GetAll(filter)
	if err != nil {
		s.errorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	s.jsonResponse(w, http.StatusOK, map[string]interface{}{
		"total": total,
		"count": len(foods),
		"items": foods,
	})
}

func (s *Server) handleSearchFoods(w http.ResponseWriter, r *http.Request) {
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 {
		limit = 10
	}

	results, err := s.storage.Search(q, limit)
	if err != nil {
		s.errorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	s.jsonResponse(w, http.StatusOK, map[string]interface{}{
		"query":   q,
		"count":   len(results),
		"results": results,
	})
}

func (s *Server) handleGetFoodByID(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		s.errorResponse(w, http.StatusBadRequest, "ID requerido")
		return
	}

	food, err := s.storage.GetByID(id)
	if err != nil {
		s.errorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	if food == nil {
		s.errorResponse(w, http.StatusNotFound, "Alimento no encontrado")
		return
	}

	s.jsonResponse(w, http.StatusOK, food)
}

func (s *Server) handleUpsertFood(w http.ResponseWriter, r *http.Request) {
	var food model.PrimaryFood
	if err := json.NewDecoder(r.Body).Decode(&food); err != nil {
		s.errorResponse(w, http.StatusBadRequest, "JSON inválido: "+err.Error())
		return
	}

	if food.Name == "" {
		s.errorResponse(w, http.StatusBadRequest, "El campo 'name' es obligatorio")
		return
	}

	if err := s.storage.Upsert(food); err != nil {
		s.errorResponse(w, http.StatusInternalServerError, "Error guardando alimento: "+err.Error())
		return
	}

	s.jsonResponse(w, http.StatusOK, map[string]interface{}{
		"status":  "saved",
		"id":      food.ID,
		"message": "Alimento guardado correctamente",
	})
}

func (s *Server) handleExportJSON(w http.ResponseWriter, r *http.Request) {
	foods, err := s.storage.ExportAll()
	if err != nil {
		s.errorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Content-Disposition", "attachment; filename=\"primary_foods.json\"")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(foods)
}

func (s *Server) handleSyncBuscatureceta(w http.ResponseWriter, r *http.Request) {
	var pkg model.BuscaturecetaPackage
	if err := json.NewDecoder(r.Body).Decode(&pkg); err != nil {
		s.errorResponse(w, http.StatusBadRequest, "JSON inválido de paquete Buscatureceta: "+err.Error())
		return
	}

	if err := s.storage.SavePantryItems(pkg.Pantry); err != nil {
		s.errorResponse(w, http.StatusInternalServerError, "Error guardando despensa: "+err.Error())
		return
	}

	if err := s.storage.SaveDiaryHistory(pkg.Diary, pkg.DiaryVersions); err != nil {
		s.errorResponse(w, http.StatusInternalServerError, "Error guardando diario: "+err.Error())
		return
	}

	s.jsonResponse(w, http.StatusOK, map[string]interface{}{
		"ok":                 true,
		"message":            "Paquete de Buscatureceta sincronizado con éxito",
		"app":                pkg.App,
		"exportedAt":         pkg.ExportedAt,
		"pantryItemsCount":   len(pkg.Pantry),
		"diaryEntriesCount":  len(pkg.Diary),
		"diaryVersionsCount": len(pkg.DiaryVersions),
	})
}

func (s *Server) handleGetPantry(w http.ResponseWriter, r *http.Request) {
	items, err := s.storage.GetPantryItems()
	if err != nil {
		s.errorResponse(w, http.StatusInternalServerError, "Error obteniendo despensa: "+err.Error())
		return
	}
	s.jsonResponse(w, http.StatusOK, map[string]interface{}{
		"total": len(items),
		"items": items,
	})
}

// handleGetConfig devuelve la configuración actual de la aplicación
func (s *Server) handleGetConfig(w http.ResponseWriter, r *http.Request) {
	s.jsonResponse(w, http.StatusOK, map[string]interface{}{
		"mealieUrl":   s.cfg.MealieURL,
		"mealieToken": s.cfg.MealieToken,
		"port":        s.cfg.Port,
		"dbPath":      s.cfg.DBPath,
		"configPath":  s.cfg.ConfigPath,
	})
}

// handleUpdateConfig actualiza la configuración en caliente y la persiste en config.json
func (s *Server) handleUpdateConfig(w http.ResponseWriter, r *http.Request) {
	var req struct {
		MealieURL   string `json:"mealieUrl"`
		MealieToken string `json:"mealieToken"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.errorResponse(w, http.StatusBadRequest, "JSON inválido: "+err.Error())
		return
	}

	if req.MealieURL != "" {
		s.cfg.MealieURL = strings.TrimRight(strings.TrimSpace(req.MealieURL), "/")
	}
	if req.MealieToken != "" {
		s.cfg.MealieToken = strings.TrimSpace(req.MealieToken)
	}

	s.mealieClient = mealie.NewClient(s.cfg.MealieURL, s.cfg.MealieToken)
	if err := config.SaveConfig(s.cfg.ConfigPath, s.cfg); err != nil {
		s.errorResponse(w, http.StatusInternalServerError, "Error guardando configuración: "+err.Error())
		return
	}

	s.jsonResponse(w, http.StatusOK, map[string]interface{}{
		"status": "ok",
		"config": s.cfg,
	})
}

func (s *Server) handleMealieStatus(w http.ResponseWriter, r *http.Request) {
	err := s.mealieClient.TestConnection(r.Context())
	recipes, _ := s.storage.GetAllRecipes()

	res := map[string]interface{}{
		"connected":     err == nil,
		"mealieUrl":     s.mealieClient.BaseURL,
		"cachedRecipes": len(recipes),
	}
	if err != nil {
		res["error"] = err.Error()
	}
	s.jsonResponse(w, http.StatusOK, res)
}

type MealieDiffItem struct {
	Slug              string   `json:"slug"`
	Name              string   `json:"name"`
	Description       string   `json:"description,omitempty"`
	Yield             int      `json:"yield,omitempty"`
	Tags              []string `json:"tags,omitempty"`
	IngredientsCount  int      `json:"ingredientsCount"`
	InstructionsCount int      `json:"instructionsCount"`
	Status            string   `json:"status"` // "new_in_mealie", "in_both", "only_in_primary"
	InMealie          bool     `json:"inMealie"`
	InPrimary         bool     `json:"inPrimary"`
	MealieURL         string   `json:"mealieUrl,omitempty"`
}

type MealieDiffResult struct {
	TotalMealie        int              `json:"totalMealie"`
	TotalPrimary       int              `json:"totalPrimary"`
	NewInMealieCount   int              `json:"newInMealieCount"`
	InBothCount        int              `json:"inBothCount"`
	OnlyInPrimaryCount int              `json:"onlyInPrimaryCount"`
	MealieBaseURL      string           `json:"mealieBaseUrl"`
	Connected          bool             `json:"connected"`
	Items              []MealieDiffItem `json:"items"`
}

func (s *Server) getMealieDiff(ctx context.Context) (*MealieDiffResult, error) {
	primaryRecipes, err := s.storage.GetAllRecipes()
	if err != nil {
		return nil, fmt.Errorf("error obteniendo recetas locales: %w", err)
	}

	mealieRecipes, err := s.mealieClient.GetRecipes(ctx)
	connected := err == nil
	if err != nil {
		mealieRecipes = []model.Recipe{}
	}

	matchedPrimary := make(map[string]bool)
	var items []MealieDiffItem

	// 1. Analizar recetas procedentes de Mealie
	for _, m := range mealieRecipes {
		var matched *model.Recipe

		// Prioridad 1: Match exacto por ID o Slug
		for i, p := range primaryRecipes {
			if matchedPrimary[p.ID] {
				continue
			}
			if m.ID == p.ID || m.Slug == p.Slug {
				matched = &primaryRecipes[i]
				matchedPrimary[p.ID] = true
				break
			}
		}

		// Prioridad 2: Match exacto por Nombre (case-insensitive)
		if matched == nil {
			for i, p := range primaryRecipes {
				if matchedPrimary[p.ID] {
					continue
				}
				if strings.EqualFold(strings.TrimSpace(m.Name), strings.TrimSpace(p.Name)) {
					matched = &primaryRecipes[i]
					matchedPrimary[p.ID] = true
					break
				}
			}
		}

		// Prioridad 3: Variantes de slug con sufijo -1, -2 (ej: "uramaki-o-rollito-invertido-1" con "uramaki-o-rollito-invertido")
		if matched == nil {
			mSlugClean := strings.TrimSuffix(strings.TrimSuffix(m.Slug, "-1"), "-2")
			for i, p := range primaryRecipes {
				if matchedPrimary[p.ID] {
					continue
				}
				pSlugClean := strings.TrimSuffix(strings.TrimSuffix(p.Slug, "-1"), "-2")
				if mSlugClean != "" && mSlugClean == pSlugClean {
					matched = &primaryRecipes[i]
					matchedPrimary[p.ID] = true
					break
				}
			}
		}

		item := MealieDiffItem{
			Slug:              m.Slug,
			Name:              m.Name,
			Description:       m.Description,
			Yield:             m.Yield,
			Tags:              m.Tags,
			IngredientsCount:  len(m.Ingredients),
			InstructionsCount: len(m.Instructions),
			InMealie:          true,
			MealieURL:         fmt.Sprintf("%s/g/home/r/%s", s.mealieClient.BaseURL, m.Slug),
		}

		if matched != nil {
			item.Status = "in_both"
			item.InPrimary = true
		} else {
			item.Status = "new_in_mealie"
			item.InPrimary = false
		}
		items = append(items, item)
	}

	// 2. Analizar recetas locales en PrimaryFoods que aún no están en Mealie
	for _, p := range primaryRecipes {
		if matchedPrimary[p.ID] {
			continue
		}
		items = append(items, MealieDiffItem{
			Slug:              p.Slug,
			Name:              p.Name,
			Description:       p.Description,
			Yield:             p.Yield,
			Tags:              p.Tags,
			IngredientsCount:  len(p.Ingredients),
			InstructionsCount: len(p.Instructions),
			Status:            "only_in_primary",
			InMealie:          false,
			InPrimary:         true,
			MealieURL:         fmt.Sprintf("%s/g/home/r/%s", s.mealieClient.BaseURL, p.Slug),
		})
	}

	// 3. Ordenar: "new_in_mealie" primero, luego "only_in_primary", luego "in_both"
	sort.SliceStable(items, func(i, j int) bool {
		rank := func(status string) int {
			switch status {
			case "new_in_mealie":
				return 0
			case "only_in_primary":
				return 1
			case "in_both":
				return 2
			default:
				return 3
			}
		}
		if rank(items[i].Status) != rank(items[j].Status) {
			return rank(items[i].Status) < rank(items[j].Status)
		}
		return items[i].Name < items[j].Name
	})

	newInMealie := 0
	inBoth := 0
	onlyInPrimary := 0
	for _, it := range items {
		switch it.Status {
		case "new_in_mealie":
			newInMealie++
		case "in_both":
			inBoth++
		case "only_in_primary":
			onlyInPrimary++
		}
	}

	return &MealieDiffResult{
		TotalMealie:        len(mealieRecipes),
		TotalPrimary:       len(primaryRecipes),
		NewInMealieCount:   newInMealie,
		InBothCount:        inBoth,
		OnlyInPrimaryCount: onlyInPrimary,
		MealieBaseURL:      s.mealieClient.BaseURL,
		Connected:          connected,
		Items:              items,
	}, nil
}

// handleMealieDiff devuelve la comparativa de recetas entre Mealie y PrimaryFoods
func (s *Server) handleMealieDiff(w http.ResponseWriter, r *http.Request) {
	diff, err := s.getMealieDiff(r.Context())
	if err != nil {
		s.errorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.jsonResponse(w, http.StatusOK, diff)
}

// handleMealieImport importa recetas desde Mealie hacia PrimaryFoods (individual o lote)
func (s *Server) handleMealieImport(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	slug := r.URL.Query().Get("slug")

	var slugsToImport []string
	if slug != "" {
		slugsToImport = append(slugsToImport, slug)
	} else if r.Body != nil {
		var req struct {
			Slugs  []string `json:"slugs"`
			AllNew bool     `json:"allNew"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		if len(req.Slugs) > 0 {
			slugsToImport = req.Slugs
		} else if req.AllNew {
			diff, err := s.getMealieDiff(ctx)
			if err == nil {
				for _, it := range diff.Items {
					if it.Status == "new_in_mealie" {
						slugsToImport = append(slugsToImport, it.Slug)
					}
				}
			}
		}
	}

	if len(slugsToImport) == 0 {
		s.errorResponse(w, http.StatusBadRequest, "Debe especificar 'slug', lista de 'slugs' o 'allNew: true'")
		return
	}

	var imported []map[string]interface{}
	var failed []map[string]interface{}

	for _, sSlug := range slugsToImport {
		detail, err := s.mealieClient.GetRecipeDetail(ctx, sSlug)
		if err != nil {
			failed = append(failed, map[string]interface{}{
				"slug":  sSlug,
				"error": err.Error(),
			})
			continue
		}

		recipe := mealie.ConvertMealieToModel(*detail)
		if recipe.ID == "" {
			recipe.ID = detail.ID
		}
		if recipe.ID == "" {
			recipe.ID = recipe.Slug
		}

		if err := s.storage.SaveRecipes([]model.Recipe{recipe}); err != nil {
			failed = append(failed, map[string]interface{}{
				"slug":  sSlug,
				"error": "Error guardando en base de datos: " + err.Error(),
			})
			continue
		}

		// Analizar la receta con el motor BEDCA / OFF
		analysis, _ := s.analyzer.AnalyzeRecipe(recipe)

		imported = append(imported, map[string]interface{}{
			"slug":             recipe.Slug,
			"name":             recipe.Name,
			"ingredientsCount": len(recipe.Ingredients),
			"matchPercentage":  analysis.MatchPercentage,
			"primaryCount":     analysis.PrimaryCount,
			"commercialCount":  analysis.CommercialCount,
			"unresolvedCount":  analysis.UnresolvedCount,
		})
	}

	s.jsonResponse(w, http.StatusOK, map[string]interface{}{
		"status":        "ok",
		"totalTarget":   len(slugsToImport),
		"totalImported": len(imported),
		"totalFailed":   len(failed),
		"imported":      imported,
		"failed":        failed,
	})
}

func (s *Server) handleMealieSync(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// 1. Intentar sincronización directa vía API de Mealie
	recipes, err := s.mealieClient.GetRecipes(ctx)
	if err != nil {
		// 2. Si Mealie está offline, intentar cargar recetas desde la carpeta sandbox/gemini/mealie/recetas-generadas/
		sandboxDir := filepath.Join("..", "sandbox", "gemini", "mealie", "recetas-generadas")
		entries, dirErr := os.ReadDir(sandboxDir)
		if dirErr == nil && len(entries) > 0 {
			var loaded []model.Recipe
			for _, entry := range entries {
				if strings.HasSuffix(entry.Name(), ".json") {
					path := filepath.Join(sandboxDir, entry.Name())
					if fileRecipes, fileErr := mealie.ParseRecipesFromJSONFile(path); fileErr == nil {
						loaded = append(loaded, fileRecipes...)
					}
				}
			}
			if len(loaded) > 0 {
				recipes = loaded
				err = nil
			}
		}
	}

	if err != nil {
		s.errorResponse(w, http.StatusBadGateway, fmt.Sprintf("No se pudo sincronizar con Mealie (%v) ni se encontraron archivos locales", err))
		return
	}

	if err := s.storage.SaveRecipes(recipes); err != nil {
		s.errorResponse(w, http.StatusInternalServerError, "Error guardando recetas en caché: "+err.Error())
		return
	}

	s.jsonResponse(w, http.StatusOK, map[string]interface{}{
		"ok":            true,
		"message":       fmt.Sprintf("Sincronizadas %d recetas", len(recipes)),
		"syncedRecipes": len(recipes),
		"recipes":       recipes,
	})
}

// handleMealieExport exporta o sube recetas hacia la instancia de Mealie
func (s *Server) handleMealieExport(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	slug := r.URL.Query().Get("slug")

	allRecipes, err := s.storage.GetAllRecipes()
	if err != nil {
		s.errorResponse(w, http.StatusInternalServerError, "Error obteniendo recetas de base de datos: "+err.Error())
		return
	}

	var targetRecipes []model.Recipe
	if slug != "" {
		for _, rec := range allRecipes {
			if rec.Slug == slug || rec.ID == slug {
				targetRecipes = append(targetRecipes, rec)
				break
			}
		}
		if len(targetRecipes) == 0 {
			s.errorResponse(w, http.StatusNotFound, "Receta no encontrada: "+slug)
			return
		}
	} else if r.Body != nil {
		var bodyObj struct {
			Slug    string         `json:"slug"`
			Recipe  *model.Recipe  `json:"recipe"`
			Recipes []model.Recipe `json:"recipes"`
		}
		if err := json.NewDecoder(r.Body).Decode(&bodyObj); err == nil {
			if bodyObj.Recipe != nil {
				targetRecipes = append(targetRecipes, *bodyObj.Recipe)
			} else if len(bodyObj.Recipes) > 0 {
				targetRecipes = bodyObj.Recipes
			} else if bodyObj.Slug != "" {
				for _, rec := range allRecipes {
					if rec.Slug == bodyObj.Slug || rec.ID == bodyObj.Slug {
						targetRecipes = append(targetRecipes, rec)
						break
					}
				}
			}
		}
	}

	if len(targetRecipes) == 0 {
		targetRecipes = allRecipes
	}

	type ExportedItem struct {
		Name string `json:"name"`
		Slug string `json:"slug"`
		URL  string `json:"url"`
	}
	type FailedItem struct {
		Name  string `json:"name"`
		Error string `json:"error"`
	}

	var exported []ExportedItem
	var failed []FailedItem

	for _, rec := range targetRecipes {
		mealieSlug, err := s.mealieClient.CreateOrUpdateRecipe(ctx, rec)
		if err != nil {
			failed = append(failed, FailedItem{
				Name:  rec.Name,
				Error: err.Error(),
			})
		} else {
			exported = append(exported, ExportedItem{
				Name: rec.Name,
				Slug: mealieSlug,
				URL:  fmt.Sprintf("%s/g/home/r/%s", s.mealieClient.BaseURL, mealieSlug),
			})
		}
	}

	s.jsonResponse(w, http.StatusOK, map[string]interface{}{
		"status":        "ok",
		"totalTarget":   len(targetRecipes),
		"totalExported": len(exported),
		"totalFailed":   len(failed),
		"exported":      exported,
		"failed":        failed,
		"mealieBaseUrl": s.mealieClient.BaseURL,
	})
}

func (s *Server) handleGetRecipesMatching(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	readyOnly := q.Get("ready_to_cook") == "true" || q.Get("ready") == "true"
	maxMissing := -1
	if m := q.Get("max_missing"); m != "" {
		if val, err := strconv.Atoi(m); err == nil {
			maxMissing = val
		}
	}
	minPct := 0.0
	if p := q.Get("min_percentage"); p != "" {
		if val, err := strconv.ParseFloat(p, 64); err == nil {
			minPct = val
		}
	}

	pantry, err := s.storage.GetPantryItems()
	if err != nil {
		s.errorResponse(w, http.StatusInternalServerError, "Error obteniendo despensa: "+err.Error())
		return
	}

	recipes, err := s.storage.GetAllRecipes()
	if err != nil {
		s.errorResponse(w, http.StatusInternalServerError, "Error obteniendo recetas: "+err.Error())
		return
	}

	matches := s.matcher.MatchRecipes(recipes, pantry, matcher.MatchOptions{
		ReadyToCookOnly:    readyOnly,
		MaxMissing:         maxMissing,
		MinMatchPercentage: minPct,
	})

	readyCount := 0
	for _, m := range matches {
		if m.IsReadyToCook {
			readyCount++
		}
	}

	s.jsonResponse(w, http.StatusOK, map[string]interface{}{
		"totalEvaluated":   len(recipes),
		"totalMatches":     len(matches),
		"readyToCookCount": readyCount,
		"pantryItemsCount": len(pantry),
		"matches":          matches,
	})
}

func (s *Server) handlePostRecipesMatching(w http.ResponseWriter, r *http.Request) {
	type adhocRequest struct {
		Pantry             []model.PantryItem `json:"pantry"`
		Recipes            []model.Recipe     `json:"recipes"`
		ReadyToCookOnly    bool               `json:"readyToCookOnly"`
		MaxMissing         *int               `json:"maxMissing"`
		MinMatchPercentage float64            `json:"minMatchPercentage"`
	}

	var req adhocRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.errorResponse(w, http.StatusBadRequest, "JSON inválido: "+err.Error())
		return
	}

	pantry := req.Pantry
	if len(pantry) == 0 {
		var err error
		pantry, err = s.storage.GetPantryItems()
		if err != nil {
			s.errorResponse(w, http.StatusInternalServerError, "Error obteniendo despensa: "+err.Error())
			return
		}
	}

	recipes := req.Recipes
	if len(recipes) == 0 {
		var err error
		recipes, err = s.storage.GetAllRecipes()
		if err != nil {
			s.errorResponse(w, http.StatusInternalServerError, "Error obteniendo recetas: "+err.Error())
			return
		}
	}

	maxMissing := -1
	if req.MaxMissing != nil {
		maxMissing = *req.MaxMissing
	}

	matches := s.matcher.MatchRecipes(recipes, pantry, matcher.MatchOptions{
		ReadyToCookOnly:    req.ReadyToCookOnly,
		MaxMissing:         maxMissing,
		MinMatchPercentage: req.MinMatchPercentage,
	})

	readyCount := 0
	for _, m := range matches {
		if m.IsReadyToCook {
			readyCount++
		}
	}

	s.jsonResponse(w, http.StatusOK, map[string]interface{}{
		"totalEvaluated":   len(recipes),
		"totalMatches":     len(matches),
		"readyToCookCount": readyCount,
		"pantryItemsCount": len(pantry),
		"matches":          matches,
	})
}

// handleImportRecipeJSON importa recetas desde un archivo o payload JSON (Qwen-VL o Schema.org)
func (s *Server) handleImportRecipeJSON(w http.ResponseWriter, r *http.Request) {
	var recipes []model.Recipe
	var err error

	filePath := r.URL.Query().Get("file")
	if filePath != "" {
		recipes, err = importer.LoadRecipesFromJSONFile(filePath)
	} else if r.Body != nil {
		recipes, err = importer.LoadRecipesFromJSON(r.Body)
	}

	if err != nil {
		s.errorResponse(w, http.StatusBadRequest, "Error procesando recetas JSON: "+err.Error())
		return
	}

	if len(recipes) == 0 {
		s.errorResponse(w, http.StatusBadRequest, "No se encontraron recetas válidas en el JSON")
		return
	}

	if err := s.storage.SaveRecipes(recipes); err != nil {
		s.errorResponse(w, http.StatusInternalServerError, "Error guardando recetas: "+err.Error())
		return
	}

	s.jsonResponse(w, http.StatusOK, map[string]interface{}{
		"status":   "ok",
		"imported": len(recipes),
		"recipes":  recipes,
	})
}

// handleAnalyzeRecipes analiza recetas identificando alimentos primarios y alternativas comerciales OFF
func (s *Server) handleAnalyzeRecipes(w http.ResponseWriter, r *http.Request) {
	slug := r.URL.Query().Get("slug")
	allRecipes, err := s.storage.GetAllRecipes()
	if err != nil {
		s.errorResponse(w, http.StatusInternalServerError, "Error obteniendo recetas: "+err.Error())
		return
	}

	var targetRecipes []model.Recipe
	if slug != "" {
		for _, rec := range allRecipes {
			if rec.Slug == slug || rec.ID == slug {
				targetRecipes = append(targetRecipes, rec)
				break
			}
		}
		if len(targetRecipes) == 0 {
			s.errorResponse(w, http.StatusNotFound, "Receta no encontrada: "+slug)
			return
		}
	} else {
		targetRecipes = allRecipes
	}

	var analyses []model.RecipeAnalysis
	totIng := 0
	totPrim := 0
	totComm := 0
	totUnres := 0

	for _, rec := range targetRecipes {
		analysis, err := s.analyzer.AnalyzeRecipe(rec)
		if err == nil {
			analyses = append(analyses, analysis)
			totIng += analysis.TotalIngredients
			totPrim += analysis.PrimaryCount
			totComm += analysis.CommercialCount
			totUnres += analysis.UnresolvedCount
		}
	}

	globalPct := 0.0
	if totIng > 0 {
		globalPct = (float64(totPrim+totComm) / float64(totIng)) * 100.0
	}

	s.jsonResponse(w, http.StatusOK, map[string]interface{}{
		"totalRecipes":          len(analyses),
		"totalIngredients":      totIng,
		"primaryMatches":        totPrim,
		"commercialMatches":     totComm,
		"unresolvedCount":       totUnres,
		"globalMatchPercentage": globalPct,
		"analyses":              analyses,
	})
}

// handleSearchProducts busca productos comerciales en el catálogo indexado de OpenFoodFacts
func (s *Server) handleSearchProducts(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	limitStr := r.URL.Query().Get("limit")
	limit := 10
	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			limit = l
		}
	}

	prods, err := s.storage.SearchCommercialProducts(q, limit)
	if err != nil {
		s.errorResponse(w, http.StatusInternalServerError, "Error buscando productos comerciales: "+err.Error())
		return
	}

	s.jsonResponse(w, http.StatusOK, map[string]interface{}{
		"query":    q,
		"total":    len(prods),
		"products": prods,
	})
}

// handleImportOFF indexa productos comerciales desde spain_products.tsv.zz
func (s *Server) handleImportOFF(w http.ResponseWriter, r *http.Request) {
	filePath := r.URL.Query().Get("file")
	if filePath == "" {
		candidates := []string{
			"../buscatureceta/src/public/spain_products.tsv.zz",
			"../../buscatureceta/src/public/spain_products.tsv.zz",
			"/home/jose/workspace/josejuanmontiel/buscatureceta/src/public/spain_products.tsv.zz",
		}
		for _, c := range candidates {
			if _, err := os.Stat(c); err == nil {
				filePath = c
				break
			}
		}
	}

	if filePath == "" {
		s.errorResponse(w, http.StatusBadRequest, "No se encontró archivo spain_products.tsv.zz")
		return
	}

	maxStr := r.URL.Query().Get("limit")
	maxProds := 50000 // Por defecto 50.000 para rapidez inicial
	if maxStr != "" {
		if m, err := strconv.Atoi(maxStr); err == nil {
			maxProds = m
		}
	}

	count, err := importer.ImportOFFProducts(importer.ImportOFFOptions{
		FilePath:            filePath,
		MaxProducts:         maxProds,
		OnlyWithIngredients: true,
		BatchSize:           2000,
	}, func(batch []model.CommercialProduct) error {
		return s.storage.SaveCommercialProducts(batch)
	})

	if err != nil {
		s.errorResponse(w, http.StatusInternalServerError, "Error importando OFF: "+err.Error())
		return
	}

	totalNow, _ := s.storage.CountCommercialProducts()
	s.jsonResponse(w, http.StatusOK, map[string]interface{}{
		"status":     "ok",
		"imported":   count,
		"totalInDB":  totalNow,
		"sourceFile": filePath,
	})
}

// handleSaveMapping guarda una asociación persistente de ingrediente
func (s *Server) handleSaveMapping(w http.ResponseWriter, r *http.Request) {
	var m model.IngredientMapping
	if err := json.NewDecoder(r.Body).Decode(&m); err != nil {
		s.errorResponse(w, http.StatusBadRequest, "JSON inválido: "+err.Error())
		return
	}

	if strings.TrimSpace(m.RawTerm) == "" || strings.TrimSpace(m.CanonicalPrimaryID) == "" {
		s.errorResponse(w, http.StatusBadRequest, "rawTerm y canonicalPrimaryId son requeridos")
		return
	}

	if err := s.storage.SaveIngredientMapping(m); err != nil {
		s.errorResponse(w, http.StatusInternalServerError, "Error guardando mapeo: "+err.Error())
		return
	}

	s.jsonResponse(w, http.StatusOK, map[string]interface{}{
		"status": "ok",
		"saved":  m,
	})
}

// handleDeconstructProduct devuelve la deconstrucción analítica y aditivos de un producto comercial
func (s *Server) handleDeconstructProduct(w http.ResponseWriter, r *http.Request) {
	code := r.PathValue("code")
	if code == "" {
		s.errorResponse(w, http.StatusBadRequest, "Código EAN requerido")
		return
	}

	prod, err := s.storage.GetCommercialProductByCode(code)
	if err != nil {
		s.errorResponse(w, http.StatusInternalServerError, "Error consultando producto: "+err.Error())
		return
	}
	if prod == nil {
		s.errorResponse(w, http.StatusNotFound, "Producto comercial no encontrado: "+code)
		return
	}

	decon := s.labelParser.DeconstructIngredients(*prod)
	s.jsonResponse(w, http.StatusOK, decon)
}

// handleGetProductsByBase busca productos comerciales que contengan como base primaria un alimento concreto
func (s *Server) handleGetProductsByBase(w http.ResponseWriter, r *http.Request) {
	primary := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("primary")))
	if primary == "" {
		s.errorResponse(w, http.StatusBadRequest, "Parámetro 'primary' requerido (ej: ?primary=garbanzo o ?primary=tomate)")
		return
	}

	limitStr := r.URL.Query().Get("limit")
	limit := 20
	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			limit = l
		}
	}

	mainOnly := r.URL.Query().Get("main") == "true" || r.URL.Query().Get("main_only") == "true"
	minPurity := 0.0
	if pStr := r.URL.Query().Get("min_purity"); pStr != "" {
		if p, err := strconv.ParseFloat(pStr, 64); err == nil {
			minPurity = p
		}
	}

	// 1. Intentar consulta ultrarrápida indexada en product_primary_bases
	indexedResults, err := s.storage.GetProductsByPrimaryFoodBase(primary, mainOnly, minPurity, limit)
	if err == nil && len(indexedResults) > 0 {
		s.jsonResponse(w, http.StatusOK, map[string]interface{}{
			"primary":    primary,
			"source":     "indexed_bases",
			"total":      len(indexedResults),
			"products":   indexedResults,
		})
		return
	}

	// 2. Fallback de escaneo heurístico si aún no está indexado
	candidates, err := s.storage.SearchCommercialProducts(primary, 50)
	if err != nil {
		s.errorResponse(w, http.StatusInternalServerError, "Error buscando productos: "+err.Error())
		return
	}

	var results []model.LabelDeconstruction
	for _, p := range candidates {
		if p.IngredientsText == "" {
			continue
		}
		decon := s.labelParser.DeconstructIngredients(p)
		matchesBase := strings.Contains(strings.ToLower(decon.MainBase), primary)
		if !matchesBase && !mainOnly {
			for _, b := range decon.TopBases {
				if strings.Contains(strings.ToLower(b), primary) {
					matchesBase = true
					break
				}
			}
		}

		if matchesBase && decon.EstimatedPurityScore >= minPurity {
			results = append(results, decon)
			if len(results) >= limit {
				break
			}
		}
	}

	s.jsonResponse(w, http.StatusOK, map[string]interface{}{
		"primary":  primary,
		"source":   "label_scan",
		"total":    len(results),
		"products": results,
	})
}

// Estructuras de exportación enriquecida para Buscatureceta
type BuscaturecetaEnrichedExport struct {
	App                     string                         `json:"app"`
	Version                 string                         `json:"version"`
	ExportedAt              string                         `json:"exportedAt"`
	Source                  string                         `json:"source"`
	TotalRecipes            int                            `json:"totalRecipes"`
	Stats                   map[string]interface{}         `json:"stats"`
	Recipes                 []BuscaturecetaEnrichedRecipe  `json:"recipes"`
	PrimaryFoodsCount       int                            `json:"primaryFoodsCount"`
	CommercialProductsCount int                            `json:"commercialProductsCount"`
}

type BuscaturecetaEnrichedRecipe struct {
	Name         string                            `json:"name"`
	Category     string                            `json:"category"`
	Servings     int                               `json:"servings"`
	Description  string                            `json:"description"`
	Instructions string                            `json:"instructions"`
	Tags         []string                          `json:"tags"`
	TotalTime    string                            `json:"totalTime,omitempty"`
	Slug         string                            `json:"slug,omitempty"`
	Image        string                            `json:"image,omitempty"`
	PrimaryFoods []string                          `json:"primaryFoods"`
	Ingredients  []BuscaturecetaEnrichedIngredient `json:"ingredients"`
}

type BuscaturecetaEnrichedIngredient struct {
	Name           string                     `json:"name"`
	Amount         float64                    `json:"amount"`
	Unit           string                     `json:"unit"`
	PrimaryFoodID  string                     `json:"primaryFoodId,omitempty"`
	PrimaryName    string                     `json:"primaryName,omitempty"`
	Category       string                     `json:"category,omitempty"`
	IsCommercial   bool                       `json:"isCommercial"`
	CommercialCode string                     `json:"commercialCode,omitempty"`
	CommercialName string                     `json:"commercialName,omitempty"`
	Alternatives   []CommercialProductSummary `json:"alternatives,omitempty"`
}

type CommercialProductSummary struct {
	Code          string  `json:"code"`
	Name          string  `json:"name"`
	Brands        string  `json:"brands,omitempty"`
	Nutriscore    string  `json:"nutriscore,omitempty"`
	PurityScore   float64 `json:"purityScore"`
	IsPrimaryBase bool    `json:"isPrimaryBase"`
}

// handleExportBuscatureceta exporta el catálogo de recetas enriquecido con bases primarias y alternativas OFF
func (s *Server) handleExportBuscatureceta(w http.ResponseWriter, r *http.Request) {
	allRecipes, err := s.storage.GetAllRecipes()
	if err != nil {
		s.errorResponse(w, http.StatusInternalServerError, "Error obteniendo recetas: "+err.Error())
		return
	}

	var enrichedRecipes []BuscaturecetaEnrichedRecipe
	totIng := 0
	totPrim := 0
	totComm := 0
	totUnres := 0

	// Cache para no repetir consultas de base primaria
	altCache := make(map[string][]CommercialProductSummary)

	for _, rec := range allRecipes {
		analysis, err := s.analyzer.AnalyzeRecipe(rec)
		if err != nil {
			continue
		}

		totIng += analysis.TotalIngredients
		totPrim += analysis.PrimaryCount
		totComm += analysis.CommercialCount
		totUnres += analysis.UnresolvedCount

		var primaryIDs []string
		primSet := make(map[string]bool)

		var enrichedIngs []BuscaturecetaEnrichedIngredient
		for _, ing := range analysis.Resolutions {
			enr := BuscaturecetaEnrichedIngredient{
				Name:   ing.OriginalText,
				Amount: ing.Amount,
				Unit:   ing.Unit,
			}

			if ing.Status == "primary_match" && ing.PrimaryFood != nil {
				enr.PrimaryFoodID = ing.PrimaryFood.ID
				enr.PrimaryName = ing.PrimaryFood.Name
				enr.Category = ing.PrimaryFood.Category
				if !primSet[ing.PrimaryFood.ID] {
					primSet[ing.PrimaryFood.ID] = true
					primaryIDs = append(primaryIDs, ing.PrimaryFood.ID)
				}

				// Consultar alternativas comerciales OFF para este ingrediente noble
				if alts, ok := altCache[ing.PrimaryFood.ID]; ok {
					enr.Alternatives = alts
				} else {
					prods, _ := s.storage.GetProductsByPrimaryFoodBase(ing.PrimaryFood.ID, true, 50.0, 3)
					if len(prods) == 0 {
						prods, _ = s.storage.GetProductsByPrimaryFoodBase(ing.NormalizedName, false, 40.0, 3)
					}
					var sumAlts []CommercialProductSummary
					for _, p := range prods {
						sumAlts = append(sumAlts, CommercialProductSummary{
							Code:          p.Product.Code,
							Name:          p.Product.ProductName,
							Brands:        p.Product.Brands,
							Nutriscore:    p.Product.NutriscoreGrade,
							PurityScore:   p.PurityScore,
							IsPrimaryBase: p.IsMainBase,
						})
					}
					altCache[ing.PrimaryFood.ID] = sumAlts
					enr.Alternatives = sumAlts
				}
			} else if ing.Status == "commercial_match" && len(ing.CommercialAlternatives) > 0 {
				comm := ing.CommercialAlternatives[0]
				enr.IsCommercial = true
				enr.CommercialCode = comm.Code
				enr.CommercialName = comm.ProductName
			}

			enrichedIngs = append(enrichedIngs, enr)
		}

		instrStr := strings.Join(rec.Instructions, "\n")
		cat := "principal"
		if len(rec.Tags) > 0 {
			cat = rec.Tags[0]
		}

		enrichedRecipes = append(enrichedRecipes, BuscaturecetaEnrichedRecipe{
			Name:         rec.Name,
			Category:     cat,
			Servings:     rec.Yield,
			Description:  rec.Description,
			Instructions: instrStr,
			Tags:         rec.Tags,
			TotalTime:    rec.TotalTime,
			Slug:         rec.Slug,
			Image:        rec.Image,
			PrimaryFoods: primaryIDs,
			Ingredients:  enrichedIngs,
		})
	}

	format := r.URL.Query().Get("format")
	download := r.URL.Query().Get("download") == "true"

	if download {
		w.Header().Set("Content-Disposition", "attachment; filename=\"buscatureceta_recipes_enriched.json\"")
	}

	if format == "recipes_only" {
		s.jsonResponse(w, http.StatusOK, enrichedRecipes)
		return
	}

	pCount, _ := s.storage.CountCommercialProducts()
	fCount, _ := s.storage.CountPrimaryFoods()

	globalPct := 0.0
	if totIng > 0 {
		globalPct = (float64(totPrim+totComm) / float64(totIng)) * 100.0
	}

	pack := BuscaturecetaEnrichedExport{
		App:          "buscatureceta",
		Version:      "2.1.0",
		ExportedAt:   time.Now().UTC().Format(time.RFC3339),
		Source:       "PrimaryFoods Database & Analyzer",
		TotalRecipes: len(enrichedRecipes),
		Stats: map[string]interface{}{
			"totalIngredients":      totIng,
			"primaryMatches":        totPrim,
			"commercialMatches":     totComm,
			"unresolvedCount":       totUnres,
			"globalMatchPercentage": globalPct,
		},
		Recipes:                 enrichedRecipes,
		PrimaryFoodsCount:       fCount,
		CommercialProductsCount: pCount,
	}

	s.jsonResponse(w, http.StatusOK, pack)
}

// handleEstimateShelfLife — GET /api/pantry/shelf-life/estimate
// Params (query): code, name, categories_tags, primary_category
// Devuelve la vida útil estimada y la fecha de caducidad sugerida.
func (s *Server) handleEstimateShelfLife(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")
	name := r.URL.Query().Get("name")
	categoriesTags := r.URL.Query().Get("categories_tags")
	primaryCategory := r.URL.Query().Get("primary_category")

	// Si se proporcionó un code pero no categoriesTags, intentar enriquecer desde la BD local
	if code != "" && categoriesTags == "" {
		if prod, err := s.storage.GetCommercialProductByCode(code); err == nil && prod != nil {
			if prod.CategoriesTags != "" {
				categoriesTags = prod.CategoriesTags
			}
			if name == "" && prod.ProductName != "" {
				name = prod.ProductName
			}
		}
		// Si es un alimento primario BEDCA, obtener su categoría
		if strings.HasPrefix(code, "primary:") {
			foodID := strings.TrimPrefix(code, "primary:")
			if food, err := s.storage.GetByID(foodID); err == nil && food != nil {
				if primaryCategory == "" {
					primaryCategory = food.Category
				}
				if name == "" {
					name = food.Name
				}
			}
		}
	}

	est := shelflife.EstimateShelfLife(code, name, categoriesTags, primaryCategory, time.Now())

	type response struct {
		Code                string `json:"code,omitempty"`
		Name                string `json:"name,omitempty"`
		Days                int    `json:"days"`
		ShelfClass          string `json:"shelfClass"`
		Reliability         string `json:"reliability"`
		RuleApplied         string `json:"ruleApplied"`
		SuggestedExpiryDate string `json:"suggestedExpiryDate"`
	}

	s.jsonResponse(w, http.StatusOK, response{
		Code:                code,
		Name:                name,
		Days:                est.Days,
		ShelfClass:          string(est.ShelfClass),
		Reliability:         est.Reliability,
		RuleApplied:         est.RuleApplied,
		SuggestedExpiryDate: est.SuggestedExpiryDate,
	})
}

// handleGetSeasonalCalendar — GET /api/seasonal/calendar
// Params opcionales: month (1-12), category (vegetable, fruit, fish), availability (peak, available)
func (s *Server) handleGetSeasonalCalendar(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	weekStr := q.Get("week")
	monthStr := q.Get("month")
	category := q.Get("category")
	availability := q.Get("availability")

	month := 0
	week := 0
	if weekStr != "" {
		if wNum, err := strconv.Atoi(weekStr); err == nil && wNum >= 1 && wNum <= 53 {
			week = wNum
			// Aproximación de mes según semana (4.33 semanas/mes)
			approxMonth := int(float64(wNum-1)/4.33) + 1
			if approxMonth > 12 {
				approxMonth = 12
			}
			month = approxMonth
		}
	} else if monthStr != "" {
		if m, err := strconv.Atoi(monthStr); err == nil && m >= 1 && m <= 12 {
			month = m
		}
	}

	filter := model.SeasonalCalendarFilter{
		Month:        month,
		Category:     category,
		Availability: availability,
	}

	items, err := s.storage.GetSeasonalCalendar(filter)
	if err != nil {
		s.errorResponse(w, http.StatusInternalServerError, "Error consultando calendario de estacionalidad: "+err.Error())
		return
	}

	s.jsonResponse(w, http.StatusOK, map[string]interface{}{
		"total": len(items),
		"month": month,
		"week":  week,
		"items": items,
	})
}

// handleGenerateWeeklyMenu — GET /api/menu/generate
// Params: week (1-52), year (ej: 2026/2027), prioritize_pantry (true/false)
func (s *Server) handleGenerateWeeklyMenu(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	weekStr := q.Get("week")
	yearStr := q.Get("year")
	prioritizePantry := q.Get("prioritize_pantry") == "true" || q.Get("pantry") == "true"
	fuelSavingMode := q.Get("fuel_saving") == "true" || q.Get("energy_saving") == "true"

	week := 0
	if weekStr != "" {
		if wNum, err := strconv.Atoi(weekStr); err == nil && wNum >= 1 && wNum <= 53 {
			week = wNum
		}
	}
	if week == 0 {
		_, currentWeek := time.Now().ISOWeek()
		week = currentWeek
	}

	year := time.Now().Year()
	if yearStr != "" {
		if yNum, err := strconv.Atoi(yearStr); err == nil && yNum > 2000 {
			year = yNum
		}
	}

	// 1. Obtener recetas
	recipes, err := s.storage.GetAllRecipes()
	if err != nil {
		s.errorResponse(w, http.StatusInternalServerError, "Error obteniendo recetas: "+err.Error())
		return
	}
	if len(recipes) == 0 {
		s.errorResponse(w, http.StatusNotFound, "No hay recetas disponibles en la base de datos para planificar")
		return
	}

	// 2. Obtener despensa del usuario
	pantry, _ := s.storage.GetPantryItems()

	// 3. Obtener alimentos de temporada para la semana
	approxMonth := int(float64(week-1)/4.33) + 1
	if approxMonth > 12 {
		approxMonth = 12
	}
	seasonalItems, _ := s.storage.GetSeasonalCalendar(model.SeasonalCalendarFilter{
		Month: approxMonth,
	})

	opts := model.MenuGenerationOptions{
		Year:             year,
		Week:             week,
		PrioritizePantry: prioritizePantry,
		FuelSavingMode:   fuelSavingMode,
		TargetLegumeDays: 3,
		TargetFishDays:   3,
	}

	plan, err := s.planner.GenerateWeeklyPlan(recipes, pantry, seasonalItems, opts)
	if err != nil {
		s.errorResponse(w, http.StatusInternalServerError, "Error generando menú semanal: "+err.Error())
		return
	}

	s.jsonResponse(w, http.StatusOK, plan)
}

// handlePostGenerateWeeklyMenu — POST /api/menu/generate con opciones ad-hoc
func (s *Server) handlePostGenerateWeeklyMenu(w http.ResponseWriter, r *http.Request) {
	type menuReq struct {
		Week             int                `json:"week"`
		Year             int                `json:"year"`
		PrioritizePantry bool               `json:"prioritizePantry"`
		TargetLegumeDays int                `json:"targetLegumeDays"`
		TargetFishDays   int                `json:"targetFishDays"`
		CustomRecipes    []model.Recipe     `json:"recipes,omitempty"`
		CustomPantry     []model.PantryItem `json:"pantry,omitempty"`
	}

	var req menuReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.errorResponse(w, http.StatusBadRequest, "JSON inválido: "+err.Error())
		return
	}

	recipes := req.CustomRecipes
	if len(recipes) == 0 {
		var err error
		recipes, err = s.storage.GetAllRecipes()
		if err != nil {
			s.errorResponse(w, http.StatusInternalServerError, "Error obteniendo recetas: "+err.Error())
			return
		}
	}

	pantry := req.CustomPantry
	if len(pantry) == 0 {
		pantry, _ = s.storage.GetPantryItems()
	}

	week := req.Week
	if week == 0 {
		_, currentWeek := time.Now().ISOWeek()
		week = currentWeek
	}
	year := req.Year
	if year == 0 {
		year = time.Now().Year()
	}

	approxMonth := int(float64(week-1)/4.33) + 1
	if approxMonth > 12 {
		approxMonth = 12
	}
	seasonalItems, _ := s.storage.GetSeasonalCalendar(model.SeasonalCalendarFilter{
		Month: approxMonth,
	})

	opts := model.MenuGenerationOptions{
		Year:             year,
		Week:             week,
		PrioritizePantry: req.PrioritizePantry,
		TargetLegumeDays: req.TargetLegumeDays,
		TargetFishDays:   req.TargetFishDays,
	}

	plan, err := s.planner.GenerateWeeklyPlan(recipes, pantry, seasonalItems, opts)
	if err != nil {
		s.errorResponse(w, http.StatusInternalServerError, "Error generando menú semanal: "+err.Error())
		return
	}

	s.jsonResponse(w, http.StatusOK, plan)
}

// handleGetCookingEnergyProfiles — GET /api/cooking/energy-profiles
// Params opcionales: class (zero, minimal, medium, high)
func (s *Server) handleGetCookingEnergyProfiles(w http.ResponseWriter, r *http.Request) {
	energyClass := r.URL.Query().Get("class")

	profiles, err := s.storage.GetCookingProfiles(energyClass)
	if err != nil {
		s.errorResponse(w, http.StatusInternalServerError, "Error obteniendo perfiles de gasto energético: "+err.Error())
		return
	}

	s.jsonResponse(w, http.StatusOK, map[string]interface{}{
		"total":    len(profiles),
		"filter":   energyClass,
		"profiles": profiles,
	})
}

// handleImportSeasonal — POST /api/seasonal/import
// Body JSON: {"url": "https://..."} o {"path": "..."} o array directo de items
func (s *Server) handleImportSeasonal(w http.ResponseWriter, r *http.Request) {
	type importReq struct {
		URL   string               `json:"url"`
		Path  string               `json:"path"`
		Items []model.SeasonalItem `json:"items"`
	}

	var req importReq
	data, err := io.ReadAll(r.Body)
	if err != nil {
		s.errorResponse(w, http.StatusBadRequest, "Error leyendo cuerpo de solicitud: "+err.Error())
		return
	}

	var items []model.SeasonalItem
	if err := json.Unmarshal(data, &req); err == nil && (req.URL != "" || req.Path != "" || len(req.Items) > 0) {
		if req.URL != "" {
			items, err = importer.ImportSeasonalFromSource(req.URL)
		} else if req.Path != "" {
			items, err = importer.ImportSeasonalFromSource(req.Path)
		} else {
			items = req.Items
		}
	} else {
		// Intentar parsear el body directo
		items, err = importer.ImportSeasonalFromSource("")
		if err != nil {
			// Intentar unmarshal directo
			if errDirect := json.Unmarshal(data, &items); errDirect != nil {
				s.errorResponse(w, http.StatusBadRequest, "Formato JSON no válido para importación estacional")
				return
			}
		}
	}

	if err != nil {
		s.errorResponse(w, http.StatusBadRequest, "Error procesando fuente estacional: "+err.Error())
		return
	}

	imported := 0
	for _, it := range items {
		if err := s.storage.UpsertSeasonalItem(it); err == nil {
			imported++
		}
	}

	s.jsonResponse(w, http.StatusOK, map[string]interface{}{
		"status":   "ok",
		"imported": imported,
		"total":    len(items),
	})
}

// handleImportCookingProfiles — POST /api/cooking/import
// Body JSON: {"url": "https://..."} o {"path": "..."} o array directo de perfiles
func (s *Server) handleImportCookingProfiles(w http.ResponseWriter, r *http.Request) {
	type importReq struct {
		URL      string                     `json:"url"`
		Path     string                     `json:"path"`
		Profiles []model.FoodCookingProfile `json:"profiles"`
	}

	var req importReq
	data, err := io.ReadAll(r.Body)
	if err != nil {
		s.errorResponse(w, http.StatusBadRequest, "Error leyendo cuerpo de solicitud: "+err.Error())
		return
	}

	var profiles []model.FoodCookingProfile
	if err := json.Unmarshal(data, &req); err == nil && (req.URL != "" || req.Path != "" || len(req.Profiles) > 0) {
		if req.URL != "" {
			profiles, err = importer.ImportCookingProfilesFromSource(req.URL)
		} else if req.Path != "" {
			profiles, err = importer.ImportCookingProfilesFromSource(req.Path)
		} else {
			profiles = req.Profiles
		}
	} else {
		if errDirect := json.Unmarshal(data, &profiles); errDirect != nil {
			s.errorResponse(w, http.StatusBadRequest, "Formato JSON no válido para perfiles de cocción")
			return
		}
	}

	if err != nil {
		s.errorResponse(w, http.StatusBadRequest, "Error procesando fuente de cocción: "+err.Error())
		return
	}

	imported := 0
	for _, p := range profiles {
		if err := s.storage.UpsertCookingProfile(p); err == nil {
			imported++
		}
	}

	s.jsonResponse(w, http.StatusOK, map[string]interface{}{
		"status":   "ok",
		"imported": imported,
		"total":    len(profiles),
	})
}

// handleGenerateAnnualMenu — GET /api/menu/annual
// Params: year (2026/2027), min_days (default 10), fuel_saving (true/false)
func (s *Server) handleGenerateAnnualMenu(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	yearStr := q.Get("year")
	minDaysStr := q.Get("min_days")
	fuelSaving := q.Get("fuel_saving") == "true" || q.Get("energy_saving") == "true"

	year := time.Now().Year()
	if yearStr != "" {
		if y, err := strconv.Atoi(yearStr); err == nil && y > 2000 {
			year = y
		}
	}

	minDays := 10
	if minDaysStr != "" {
		if d, err := strconv.Atoi(minDaysStr); err == nil && d > 0 {
			minDays = d
		}
	}

	recipes, err := s.storage.GetAllRecipes()
	if err != nil {
		s.errorResponse(w, http.StatusInternalServerError, "Error obteniendo recetas: "+err.Error())
		return
	}
	if len(recipes) == 0 {
		s.errorResponse(w, http.StatusNotFound, "No hay recetas disponibles en la base de datos para planificar")
		return
	}

	pantry, _ := s.storage.GetPantryItems()
	seasonalItems, _ := s.storage.GetSeasonalCalendar(model.SeasonalCalendarFilter{})

	opts := model.AnnualPlanOptions{
		Year:                  year,
		MinDaysBetweenRepeats: minDays,
		FuelSavingMode:        fuelSaving,
		TargetLegumeDays:      3,
		TargetFishDays:        3,
	}

	annualPlan, err := s.planner.GenerateAnnualPlan(recipes, pantry, seasonalItems, opts)
	if err != nil {
		s.errorResponse(w, http.StatusInternalServerError, "Error generando menú anual: "+err.Error())
		return
	}

	s.jsonResponse(w, http.StatusOK, annualPlan)
}

// handleExportAnnualMenu — GET /api/menu/annual/export
// Devuelve el formato BuscaturecetaPackage con las 52 semanas listo para importar en Buscatureceta/NutriAgenda
func (s *Server) handleExportAnnualMenu(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	yearStr := q.Get("year")
	minDaysStr := q.Get("min_days")
	fuelSaving := q.Get("fuel_saving") == "true" || q.Get("energy_saving") == "true"

	year := time.Now().Year()
	if yearStr != "" {
		if y, err := strconv.Atoi(yearStr); err == nil && y > 2000 {
			year = y
		}
	}

	minDays := 10
	if minDaysStr != "" {
		if d, err := strconv.Atoi(minDaysStr); err == nil && d > 0 {
			minDays = d
		}
	}

	recipes, err := s.storage.GetAllRecipes()
	if err != nil {
		s.errorResponse(w, http.StatusInternalServerError, "Error obteniendo recetas: "+err.Error())
		return
	}
	if len(recipes) == 0 {
		s.errorResponse(w, http.StatusNotFound, "No hay recetas disponibles para planificar")
		return
	}

	pantry, _ := s.storage.GetPantryItems()
	seasonalItems, _ := s.storage.GetSeasonalCalendar(model.SeasonalCalendarFilter{})

	opts := model.AnnualPlanOptions{
		Year:                  year,
		MinDaysBetweenRepeats: minDays,
		FuelSavingMode:        fuelSaving,
		TargetLegumeDays:      3,
		TargetFishDays:        3,
	}

	annualPlan, err := s.planner.GenerateAnnualPlan(recipes, pantry, seasonalItems, opts)
	if err != nil {
		s.errorResponse(w, http.StatusInternalServerError, "Error generando plan anual: "+err.Error())
		return
	}

	buscaturecetaPkg := planner.ExportAnnualPlanToBuscatureceta(annualPlan)

	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"plan_anual_%d_buscatureceta.json\"", year))
	s.jsonResponse(w, http.StatusOK, buscaturecetaPkg)
}


