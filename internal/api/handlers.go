package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/josejuanmontiel/primaryfoods/internal/matcher"
	"github.com/josejuanmontiel/primaryfoods/internal/mealie"
	"github.com/josejuanmontiel/primaryfoods/internal/model"
	"github.com/josejuanmontiel/primaryfoods/internal/storage"
)

// Server expone la API HTTP de PrimaryFoods
type Server struct {
	storage      *storage.Storage
	mux          *http.ServeMux
	mealieClient *mealie.Client
	matcher      *matcher.RecipeMatcher
}

// NewServer crea un nuevo servidor HTTP con todas sus rutas registradas
func NewServer(store *storage.Storage) *Server {
	foods, _ := store.ExportAll()
	mealieURL := os.Getenv("MEALIE_URL")
	if mealieURL == "" {
		mealieURL = "http://localhost:9925"
	}
	mealieToken := os.Getenv("MEALIE_TOKEN")
	if mealieToken == "" {
		mealieToken = os.Getenv("MEALIE_API_KEY")
	}
	if mealieToken == "" {
		envCandidates := []string{
			"../sandbox/gemini/mealie/.env",
			"../../sandbox/gemini/mealie/.env",
			".env",
		}
		for _, candidate := range envCandidates {
			if content, err := os.ReadFile(candidate); err == nil {
				for _, line := range strings.Split(string(content), "\n") {
					line = strings.TrimSpace(line)
					if strings.HasPrefix(line, "MEALIE_API_KEY=") {
						mealieToken = strings.TrimPrefix(line, "MEALIE_API_KEY=")
						break
					}
				}
				if mealieToken != "" {
					break
				}
			}
		}
	}

	s := &Server{
		storage:      store,
		mux:          http.NewServeMux(),
		mealieClient: mealie.NewClient(mealieURL, mealieToken),
		matcher:      matcher.NewMatcher(foods),
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

	// Sincronización Buscatureceta
	s.mux.HandleFunc("POST /api/sync/buscatureceta", s.handleSyncBuscatureceta)
	s.mux.HandleFunc("GET /api/pantry", s.handleGetPantry)

	// Conexión y sincronización Mealie
	s.mux.HandleFunc("GET /api/mealie/status", s.handleMealieStatus)
	s.mux.HandleFunc("POST /api/mealie/sync", s.handleMealieSync)

	// Motor Despensa -> Receta
	s.mux.HandleFunc("GET /api/recipes/matching", s.handleGetRecipesMatching)
	s.mux.HandleFunc("POST /api/recipes/matching", s.handlePostRecipesMatching)

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
