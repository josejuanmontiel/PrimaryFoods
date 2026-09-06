package storage

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/josejuanmontiel/primaryfoods/internal/model"
	"github.com/josejuanmontiel/primaryfoods/internal/seed"
	_ "modernc.org/sqlite"
)

// Storage maneja la persistencia de alimentos primarios en SQLite
type Storage struct {
	db *sql.DB
}

// NewSQLiteStorage inicializa la base de datos SQLite y carga los datos semilla si está vacía
func NewSQLiteStorage(dbPath string) (*Storage, error) {
	if dbPath != ":memory:" {
		dir := filepath.Dir(dbPath)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("error creando directorio de BD: %w", err)
		}
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("error abriendo SQLite: %w", err)
	}

	s := &Storage{db: db}
	if err := s.initSchema(); err != nil {
		db.Close()
		return nil, fmt.Errorf("error inicializando schema: %w", err)
	}

	// Auto-seed si la base de datos está vacía
	if err := s.seedIfEmpty(); err != nil {
		db.Close()
		return nil, fmt.Errorf("error ejecutando seed inicial: %w", err)
	}

	return s, nil
}

func (s *Storage) initSchema() error {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS foods (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			scientific_name TEXT,
			category TEXT NOT NULL,
			calories REAL DEFAULT 0,
			protein REAL DEFAULT 0,
			fat REAL DEFAULT 0,
			carbs REAL DEFAULT 0,
			fiber REAL DEFAULT 0,
			sugars REAL DEFAULT 0,
			sodium REAL DEFAULT 0,
			data_source TEXT,
			data_json TEXT NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);`,
		`CREATE TABLE IF NOT EXISTS food_aliases (
			alias TEXT NOT NULL,
			food_id TEXT NOT NULL,
			PRIMARY KEY (alias, food_id),
			FOREIGN KEY (food_id) REFERENCES foods(id) ON DELETE CASCADE
		);`,
		`CREATE TABLE IF NOT EXISTS user_pantry (
			id INTEGER PRIMARY KEY,
			product_code TEXT NOT NULL,
			product_name TEXT NOT NULL,
			amount REAL DEFAULT 0,
			unit TEXT DEFAULT '',
			pantry_zone TEXT DEFAULT 'food',
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);`,
		`CREATE TABLE IF NOT EXISTS user_diary (
			id INTEGER PRIMARY KEY,
			date TEXT NOT NULL,
			meal_type TEXT NOT NULL,
			status TEXT DEFAULT 'consumed',
			data_json TEXT NOT NULL,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);`,
		`CREATE TABLE IF NOT EXISTS user_diary_history (
			id INTEGER PRIMARY KEY,
			diary_entry_id INTEGER NOT NULL,
			date TEXT NOT NULL,
			meal_type TEXT NOT NULL,
			action TEXT NOT NULL,
			version_number INTEGER DEFAULT 1,
			data_json TEXT NOT NULL,
			timestamp TEXT NOT NULL
		);`,
		`CREATE TABLE IF NOT EXISTS mealie_recipes (
			id TEXT PRIMARY KEY,
			slug TEXT NOT NULL,
			name TEXT NOT NULL,
			data_json TEXT NOT NULL,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);`,
		`CREATE INDEX IF NOT EXISTS idx_foods_category ON foods(category);`,
		`CREATE INDEX IF NOT EXISTS idx_foods_name ON foods(name);`,
		`CREATE INDEX IF NOT EXISTS idx_aliases_alias ON food_aliases(alias);`,
		`CREATE INDEX IF NOT EXISTS idx_user_pantry_code ON user_pantry(product_code);`,
		`CREATE INDEX IF NOT EXISTS idx_user_pantry_name ON user_pantry(product_name);`,
		`CREATE INDEX IF NOT EXISTS idx_user_diary_date ON user_diary(date);`,
		`CREATE INDEX IF NOT EXISTS idx_user_diary_history_entry ON user_diary_history(diary_entry_id);`,
		`CREATE INDEX IF NOT EXISTS idx_mealie_recipes_slug ON mealie_recipes(slug);`,
	}

	for _, q := range queries {
		if _, err := s.db.Exec(q); err != nil {
			return err
		}
	}
	return nil
}

func (s *Storage) seedIfEmpty() error {
	var count int
	if err := s.db.QueryRow("SELECT COUNT(*) FROM foods").Scan(&count); err != nil {
		return err
	}

	if count > 0 {
		return nil // Ya tiene datos
	}

	defaultFoods, err := seed.GetDefaultFoods()
	if err != nil {
		return err
	}

	for _, f := range defaultFoods {
		if err := s.Upsert(f); err != nil {
			return fmt.Errorf("error insertando alimento semilla '%s': %w", f.ID, err)
		}
	}

	return nil
}

// Upsert inserta o actualiza un alimento primario y sus aliases
func (s *Storage) Upsert(f model.PrimaryFood) error {
	if f.ID == "" {
		f.ID = strings.ToLower(strings.ReplaceAll(f.Name, " ", "_"))
	}

	rawJSON, err := json.Marshal(f)
	if err != nil {
		return fmt.Errorf("error serializando alimento a JSON: %w", err)
	}

	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	query := `INSERT INTO foods (
		id, name, scientific_name, category,
		calories, protein, fat, carbs, fiber, sugars, sodium,
		data_source, data_json, updated_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
	ON CONFLICT(id) DO UPDATE SET
		name = excluded.name,
		scientific_name = excluded.scientific_name,
		category = excluded.category,
		calories = excluded.calories,
		protein = excluded.protein,
		fat = excluded.fat,
		carbs = excluded.carbs,
		fiber = excluded.fiber,
		sugars = excluded.sugars,
		sodium = excluded.sodium,
		data_source = excluded.data_source,
		data_json = excluded.data_json,
		updated_at = CURRENT_TIMESTAMP;`

	_, err = tx.Exec(
		query,
		f.ID, f.Name, f.ScientificName, f.Category,
		f.NutritionPer100g.Calories, f.NutritionPer100g.Protein, f.NutritionPer100g.Fat,
		f.NutritionPer100g.Carbs, f.NutritionPer100g.Fiber, f.NutritionPer100g.Sugars, f.NutritionPer100g.Sodium,
		f.DataSource, string(rawJSON),
	)
	if err != nil {
		return err
	}

	// Actualizar aliases
	if _, err := tx.Exec("DELETE FROM food_aliases WHERE food_id = ?", f.ID); err != nil {
		return err
	}

	// Insertar el propio nombre como alias canónico
	allAliases := append([]string{strings.ToLower(f.Name)}, f.Aliases...)
	seen := make(map[string]bool)
	for _, alias := range allAliases {
		cleaned := strings.TrimSpace(strings.ToLower(alias))
		if cleaned == "" || seen[cleaned] {
			continue
		}
		seen[cleaned] = true
		if _, err := tx.Exec("INSERT OR IGNORE INTO food_aliases (alias, food_id) VALUES (?, ?)", cleaned, f.ID); err != nil {
			return err
		}
	}

	return tx.Commit()
}

// GetByID obtiene un alimento por su identificador
func (s *Storage) GetByID(id string) (*model.PrimaryFood, error) {
	var rawJSON string
	err := s.db.QueryRow("SELECT data_json FROM foods WHERE id = ?", id).Scan(&rawJSON)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	var food model.PrimaryFood
	if err := json.Unmarshal([]byte(rawJSON), &food); err != nil {
		return nil, err
	}
	return &food, nil
}

// GetAll lista alimentos con filtrado opcional por categoría o tag
func (s *Storage) GetAll(filter model.SearchFilter) ([]model.PrimaryFood, int, error) {
	var conditions []string
	var args []interface{}

	if filter.Category != "" {
		conditions = append(conditions, "category = ?")
		args = append(args, filter.Category)
	}

	whereClause := ""
	if len(conditions) > 0 {
		whereClause = "WHERE " + strings.Join(conditions, " AND ")
	}

	// Total count
	var total int
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM foods %s", whereClause)
	if err := s.db.QueryRow(countQuery, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	limit := filter.Limit
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	offset := filter.Offset
	if offset < 0 {
		offset = 0
	}

	query := fmt.Sprintf("SELECT data_json FROM foods %s ORDER BY name ASC LIMIT ? OFFSET ?", whereClause)
	args = append(args, limit, offset)

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var foods []model.PrimaryFood
	for rows.Next() {
		var rawJSON string
		if err := rows.Scan(&rawJSON); err != nil {
			return nil, 0, err
		}
		var f model.PrimaryFood
		if err := json.Unmarshal([]byte(rawJSON), &f); err == nil {
			if filter.Tag != "" {
				// Filtrar por tag en memoria si se especificó
				hasTag := false
				for _, t := range f.Tags {
					if strings.EqualFold(t, filter.Tag) || strings.EqualFold(t, "#"+filter.Tag) {
						hasTag = true
						break
					}
				}
				if !hasTag {
					continue
				}
			}
			foods = append(foods, f)
		}
	}

	return foods, total, nil
}

// Search busca alimentos primarios por nombre o sinónimo para el Smart Match de Buscatureceta
func (s *Storage) Search(query string, limit int) ([]model.PrimaryFood, error) {
	q := strings.TrimSpace(strings.ToLower(query))
	if q == "" {
		list, _, err := s.GetAll(model.SearchFilter{Limit: limit})
		return list, err
	}

	if limit <= 0 {
		limit = 10
	}

	// Búsqueda jerárquica:
	// 1. Coincidencia exacta de alias o ID
	// 2. Prefijo de alias o nombre
	// 3. Subcadena en alias o nombre
	sqlQuery := `
	SELECT DISTINCT f.data_json,
		CASE
			WHEN a.alias = ? THEN 1
			WHEN f.id = ? THEN 1
			WHEN a.alias LIKE ? THEN 2
			WHEN f.name LIKE ? THEN 2
			ELSE 3
		END as score
	FROM foods f
	JOIN food_aliases a ON a.food_id = f.id
	WHERE a.alias LIKE ? OR f.name LIKE ? OR f.id LIKE ?
	ORDER BY score ASC, f.name ASC
	LIMIT ?;`

	prefix := q + "%"
	contains := "%" + q + "%"

	rows, err := s.db.Query(sqlQuery, q, q, prefix, prefix, contains, contains, contains, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []model.PrimaryFood
	for rows.Next() {
		var rawJSON string
		var score int
		if err := rows.Scan(&rawJSON, &score); err != nil {
			return nil, err
		}
		var f model.PrimaryFood
		if err := json.Unmarshal([]byte(rawJSON), &f); err == nil {
			results = append(results, f)
		}
	}

	return results, nil
}

// ExportAll devuelve todos los alimentos en un único slice para exportación offline
func (s *Storage) ExportAll() ([]model.PrimaryFood, error) {
	rows, err := s.db.Query("SELECT data_json FROM foods ORDER BY category, name ASC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var all []model.PrimaryFood
	for rows.Next() {
		var rawJSON string
		if err := rows.Scan(&rawJSON); err != nil {
			return nil, err
		}
		var f model.PrimaryFood
		if err := json.Unmarshal([]byte(rawJSON), &f); err == nil {
			all = append(all, f)
		}
	}
	return all, nil
}

// SavePantryItems guarda o reemplaza el inventario de despensa del usuario
func (s *Storage) SavePantryItems(items []model.PantryItem) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec("DELETE FROM user_pantry"); err != nil {
		return err
	}

	stmt, err := tx.Prepare(`INSERT INTO user_pantry (id, product_code, product_name, amount, unit, pantry_zone, updated_at) 
		VALUES (?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, item := range items {
		if _, err := stmt.Exec(item.ID, item.ProductCode, item.ProductName, item.Amount, item.Unit, item.PantryZone); err != nil {
			return err
		}
	}

	return tx.Commit()
}

// GetPantryItems devuelve todos los productos actuales de la despensa
func (s *Storage) GetPantryItems() ([]model.PantryItem, error) {
	rows, err := s.db.Query("SELECT id, product_code, product_name, amount, unit, pantry_zone FROM user_pantry ORDER BY product_name ASC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []model.PantryItem
	for rows.Next() {
		var item model.PantryItem
		if err := rows.Scan(&item.ID, &item.ProductCode, &item.ProductName, &item.Amount, &item.Unit, &item.PantryZone); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, nil
}

// SaveDiaryHistory almacena las entradas de diario y las versiones históricas de decisiones
func (s *Storage) SaveDiaryHistory(diary []model.DiaryEntry, versions []model.DiaryVersion) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec("DELETE FROM user_diary"); err != nil {
		return err
	}
	if _, err := tx.Exec("DELETE FROM user_diary_history"); err != nil {
		return err
	}

	stmtDiary, err := tx.Prepare(`INSERT INTO user_diary (id, date, meal_type, status, data_json, updated_at) VALUES (?, ?, ?, ?, ?, CURRENT_TIMESTAMP)`)
	if err != nil {
		return err
	}
	defer stmtDiary.Close()

	for _, d := range diary {
		dataJSON, err := json.Marshal(d)
		if err != nil {
			continue
		}
		if _, err := stmtDiary.Exec(d.ID, d.Date, d.MealType, d.Status, string(dataJSON)); err != nil {
			return err
		}
	}

	stmtVer, err := tx.Prepare(`INSERT INTO user_diary_history (id, diary_entry_id, date, meal_type, action, version_number, data_json, timestamp) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return err
	}
	defer stmtVer.Close()

	for _, v := range versions {
		dataJSON, err := json.Marshal(v)
		if err != nil {
			continue
		}
		if _, err := stmtVer.Exec(v.ID, v.DiaryEntryID, v.Date, v.MealType, v.Action, v.VersionNumber, string(dataJSON), v.Timestamp); err != nil {
			return err
		}
	}

	return tx.Commit()
}

// SaveRecipes guarda o actualiza recetas (de Mealie o internas) en la base de datos
func (s *Storage) SaveRecipes(recipes []model.Recipe) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`INSERT INTO mealie_recipes (id, slug, name, data_json, updated_at) 
		VALUES (?, ?, ?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(id) DO UPDATE SET slug = excluded.slug, name = excluded.name, data_json = excluded.data_json, updated_at = CURRENT_TIMESTAMP`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, r := range recipes {
		dataJSON, err := json.Marshal(r)
		if err != nil {
			continue
		}
		if _, err := stmt.Exec(r.ID, r.Slug, r.Name, string(dataJSON)); err != nil {
			return err
		}
	}

	return tx.Commit()
}

// GetAllRecipes devuelve todas las recetas indexadas en SQLite
func (s *Storage) GetAllRecipes() ([]model.Recipe, error) {
	rows, err := s.db.Query("SELECT data_json FROM mealie_recipes ORDER BY name ASC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var recipes []model.Recipe
	for rows.Next() {
		var rawJSON string
		if err := rows.Scan(&rawJSON); err != nil {
			return nil, err
		}
		var r model.Recipe
		if err := json.Unmarshal([]byte(rawJSON), &r); err == nil {
			recipes = append(recipes, r)
		}
	}
	return recipes, nil
}

// Close cierra la conexión a la base de datos
func (s *Storage) Close() error {
	return s.db.Close()
}
