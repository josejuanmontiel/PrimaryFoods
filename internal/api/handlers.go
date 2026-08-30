package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/josejuanmontiel/primaryfoods/internal/model"
	"github.com/josejuanmontiel/primaryfoods/internal/storage"
)

// Server expone la API HTTP de PrimaryFoods
type Server struct {
	storage *storage.Storage
	mux     *http.ServeMux
}

// NewServer crea un nuevo servidor HTTP con todas sus rutas registradas
func NewServer(store *storage.Storage) *Server {
	s := &Server{
		storage: store,
		mux:     http.NewServeMux(),
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
