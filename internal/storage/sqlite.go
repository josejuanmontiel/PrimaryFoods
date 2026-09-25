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
		`CREATE TABLE IF NOT EXISTS commercial_products (
			code TEXT PRIMARY KEY,
			product_name TEXT NOT NULL,
			brands TEXT,
			nutriscore_grade TEXT,
			nova_group TEXT,
			ingredients_text TEXT,
			categories_tags TEXT,
			image_url TEXT,
			energy_kcal REAL DEFAULT 0,
			protein REAL DEFAULT 0,
			fat REAL DEFAULT 0,
			carbs REAL DEFAULT 0,
			fiber REAL DEFAULT 0,
			salt REAL DEFAULT 0,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);`,
		`CREATE TABLE IF NOT EXISTS ingredient_mappings (
			raw_term TEXT PRIMARY KEY,
			canonical_primary_id TEXT NOT NULL,
			commercial_codes TEXT,
			confidence REAL DEFAULT 1.0,
			source TEXT DEFAULT 'rule',
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);`,
		`CREATE TABLE IF NOT EXISTS product_primary_bases (
			product_code TEXT NOT NULL,
			primary_food_id TEXT NOT NULL,
			position INTEGER NOT NULL,
			percentage REAL DEFAULT 0,
			is_main_base BOOLEAN DEFAULT FALSE,
			purity_score REAL DEFAULT 50.0,
			PRIMARY KEY (product_code, primary_food_id)
		);`,
		`CREATE INDEX IF NOT EXISTS idx_foods_category ON foods(category);`,
		`CREATE INDEX IF NOT EXISTS idx_foods_name ON foods(name);`,
		`CREATE INDEX IF NOT EXISTS idx_aliases_alias ON food_aliases(alias);`,
		`CREATE INDEX IF NOT EXISTS idx_user_pantry_code ON user_pantry(product_code);`,
		`CREATE INDEX IF NOT EXISTS idx_user_pantry_name ON user_pantry(product_name);`,
		`CREATE INDEX IF NOT EXISTS idx_user_diary_date ON user_diary(date);`,
		`CREATE INDEX IF NOT EXISTS idx_user_diary_history_entry ON user_diary_history(diary_entry_id);`,
		`CREATE INDEX IF NOT EXISTS idx_mealie_recipes_slug ON mealie_recipes(slug);`,
		`CREATE INDEX IF NOT EXISTS idx_comm_products_name ON commercial_products(product_name);`,
		`CREATE INDEX IF NOT EXISTS idx_comm_products_brand ON commercial_products(brands);`,
		`CREATE INDEX IF NOT EXISTS idx_ppb_primary ON product_primary_bases(primary_food_id);`,
		`CREATE INDEX IF NOT EXISTS idx_ppb_position ON product_primary_bases(position);`,
		`CREATE INDEX IF NOT EXISTS idx_ppb_main ON product_primary_bases(is_main_base);`,
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
		if r.Slug != "" {
			_, _ = tx.Exec("DELETE FROM mealie_recipes WHERE slug = ? AND id != ?", r.Slug, r.ID)
		}
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

// SaveCommercialProducts guarda o actualiza productos comerciales de OpenFoodFacts en lote
func (s *Storage) SaveCommercialProducts(products []model.CommercialProduct) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`INSERT INTO commercial_products (
		code, product_name, brands, nutriscore_grade, nova_group,
		ingredients_text, categories_tags, image_url,
		energy_kcal, protein, fat, carbs, fiber, salt, updated_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
	ON CONFLICT(code) DO UPDATE SET
		product_name = excluded.product_name,
		brands = excluded.brands,
		nutriscore_grade = excluded.nutriscore_grade,
		nova_group = excluded.nova_group,
		ingredients_text = excluded.ingredients_text,
		categories_tags = excluded.categories_tags,
		image_url = excluded.image_url,
		energy_kcal = excluded.energy_kcal,
		protein = excluded.protein,
		fat = excluded.fat,
		carbs = excluded.carbs,
		fiber = excluded.fiber,
		salt = excluded.salt,
		updated_at = CURRENT_TIMESTAMP`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, p := range products {
		if _, err := stmt.Exec(
			p.Code, p.ProductName, p.Brands, p.NutriscoreGrade, p.NovaGroup,
			p.IngredientsText, p.CategoriesTags, p.ImageURL,
			p.EnergyKcal, p.Protein, p.Fat, p.Carbs, p.Fiber, p.Salt,
		); err != nil {
			return err
		}
	}

	return tx.Commit()
}

// SearchCommercialProducts busca productos comerciales de OFF ordenando por calidad (NutriScore A/B y Nova 1/2 primero)
func (s *Storage) SearchCommercialProducts(query string, limit int) ([]model.CommercialProduct, error) {
	q := strings.TrimSpace(strings.ToLower(query))
	if q == "" {
		return nil, nil
	}
	if limit <= 0 {
		limit = 10
	}

	likeQuery := "%" + q + "%"
	sqlQuery := `
	SELECT code, product_name, brands, nutriscore_grade, nova_group,
	       ingredients_text, categories_tags, image_url,
	       energy_kcal, protein, fat, carbs, fiber, salt
	FROM commercial_products
	WHERE product_name LIKE ? OR brands LIKE ?
	ORDER BY
	  CASE
	    WHEN lower(product_name) = ? THEN 1
	    WHEN lower(product_name) LIKE ? THEN 2
	    ELSE 3
	  END ASC,
	  CASE
	    WHEN nutriscore_grade = 'a' THEN 1
	    WHEN nutriscore_grade = 'b' THEN 2
	    WHEN nutriscore_grade = 'c' THEN 3
	    WHEN nutriscore_grade = 'd' THEN 4
	    WHEN nutriscore_grade = 'e' THEN 5
	    ELSE 6
	  END ASC,
	  CASE
	    WHEN nova_group = '1' THEN 1
	    WHEN nova_group = '2' THEN 2
	    WHEN nova_group = '3' THEN 3
	    WHEN nova_group = '4' THEN 4
	    ELSE 5
	  END ASC
	LIMIT ?;`

	prefix := q + "%"
	rows, err := s.db.Query(sqlQuery, likeQuery, likeQuery, q, prefix, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var products []model.CommercialProduct
	for rows.Next() {
		var p model.CommercialProduct
		var brands, nutriscore, nova, ingr, cat, img sql.NullString
		if err := rows.Scan(
			&p.Code, &p.ProductName, &brands, &nutriscore, &nova,
			&ingr, &cat, &img,
			&p.EnergyKcal, &p.Protein, &p.Fat, &p.Carbs, &p.Fiber, &p.Salt,
		); err != nil {
			return nil, err
		}
		p.Brands = brands.String
		p.NutriscoreGrade = nutriscore.String
		p.NovaGroup = nova.String
		p.IngredientsText = ingr.String
		p.CategoriesTags = cat.String
		p.ImageURL = img.String
		products = append(products, p)
	}

	return products, nil
}

// GetCommercialProductByCode busca un producto por código EAN
func (s *Storage) GetCommercialProductByCode(code string) (*model.CommercialProduct, error) {
	sqlQuery := `
	SELECT code, product_name, brands, nutriscore_grade, nova_group,
	       ingredients_text, categories_tags, image_url,
	       energy_kcal, protein, fat, carbs, fiber, salt
	FROM commercial_products
	WHERE code = ? LIMIT 1;`

	var p model.CommercialProduct
	var brands, nutriscore, nova, ingr, cat, img sql.NullString
	err := s.db.QueryRow(sqlQuery, code).Scan(
		&p.Code, &p.ProductName, &brands, &nutriscore, &nova,
		&ingr, &cat, &img,
		&p.EnergyKcal, &p.Protein, &p.Fat, &p.Carbs, &p.Fiber, &p.Salt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	p.Brands = brands.String
	p.NutriscoreGrade = nutriscore.String
	p.NovaGroup = nova.String
	p.IngredientsText = ingr.String
	p.CategoriesTags = cat.String
	p.ImageURL = img.String
	return &p, nil
}

// CountCommercialProducts cuenta cuántos productos comerciales están indexados
func (s *Storage) CountCommercialProducts() (int, error) {
	var count int
	err := s.db.QueryRow("SELECT COUNT(*) FROM commercial_products").Scan(&count)
	return count, err
}

// CountPrimaryFoods cuenta cuántos alimentos primarios están registrados en BEDCA
func (s *Storage) CountPrimaryFoods() (int, error) {
	var count int
	err := s.db.QueryRow("SELECT COUNT(*) FROM foods").Scan(&count)
	return count, err
}


// SaveIngredientMapping guarda una asociación persistente
func (s *Storage) SaveIngredientMapping(m model.IngredientMapping) error {
	rawJSON, _ := json.Marshal(m.CommercialCodes)
	query := `INSERT INTO ingredient_mappings (raw_term, canonical_primary_id, commercial_codes, confidence, source, updated_at)
		VALUES (?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(raw_term) DO UPDATE SET
			canonical_primary_id = excluded.canonical_primary_id,
			commercial_codes = excluded.commercial_codes,
			confidence = excluded.confidence,
			source = excluded.source,
			updated_at = CURRENT_TIMESTAMP;`
	_, err := s.db.Exec(query, strings.ToLower(strings.TrimSpace(m.RawTerm)), m.CanonicalPrimaryID, string(rawJSON), m.Confidence, m.Source)
	return err
}

// GetIngredientMapping busca una asociación persistente
func (s *Storage) GetIngredientMapping(rawTerm string) (*model.IngredientMapping, error) {
	term := strings.ToLower(strings.TrimSpace(rawTerm))
	var m model.IngredientMapping
	var commCodesJSON sql.NullString
	err := s.db.QueryRow("SELECT raw_term, canonical_primary_id, commercial_codes, confidence, source FROM ingredient_mappings WHERE raw_term = ?", term).
		Scan(&m.RawTerm, &m.CanonicalPrimaryID, &commCodesJSON, &m.Confidence, &m.Source)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if commCodesJSON.Valid && commCodesJSON.String != "" {
		_ = json.Unmarshal([]byte(commCodesJSON.String), &m.CommercialCodes)
	}
	return &m, nil
}

// Close cierra la conexión a la base de datos
func (s *Storage) Close() error {
	return s.db.Close()
}

// SaveProductPrimaryBases guarda en lote las relaciones producto -> base primaria
func (s *Storage) SaveProductPrimaryBases(bases []model.ProductPrimaryBase) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`INSERT INTO product_primary_bases (
		product_code, primary_food_id, position, percentage, is_main_base, purity_score
	) VALUES (?, ?, ?, ?, ?, ?)
	ON CONFLICT(product_code, primary_food_id) DO UPDATE SET
		position = excluded.position,
		percentage = excluded.percentage,
		is_main_base = excluded.is_main_base,
		purity_score = excluded.purity_score`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, b := range bases {
		if _, err := stmt.Exec(b.ProductCode, b.PrimaryFoodID, b.Position, b.Percentage, b.IsMainBase, b.PurityScore); err != nil {
			return err
		}
	}

	return tx.Commit()
}

// GetProductsByPrimaryFoodBase busca productos comerciales indexados por base primaria en O(1) con filtros de pureza y orden
func (s *Storage) GetProductsByPrimaryFoodBase(primaryID string, mainOnly bool, minPurity float64, limit int) ([]model.CommercialProductWithBase, error) {
	if limit <= 0 {
		limit = 30
	}

	mainClause := ""
	if mainOnly {
		mainClause = "AND ppb.is_main_base = 1"
	}

	sqlQuery := fmt.Sprintf(`
	SELECT p.code, p.product_name, p.brands, p.nutriscore_grade, p.nova_group,
	       p.ingredients_text, p.categories_tags, p.image_url,
	       p.energy_kcal, p.protein, p.fat, p.carbs, p.fiber, p.salt,
	       ppb.primary_food_id, ppb.position, ppb.percentage, ppb.is_main_base, ppb.purity_score
	FROM commercial_products p
	JOIN product_primary_bases ppb ON ppb.product_code = p.code
	WHERE (ppb.primary_food_id = ? OR ppb.primary_food_id LIKE ?)
	  AND ppb.purity_score >= ?
	  %s
	ORDER BY
	  ppb.position ASC,
	  ppb.purity_score DESC,
	  CASE
	    WHEN p.nutriscore_grade = 'a' THEN 1
	    WHEN p.nutriscore_grade = 'b' THEN 2
	    WHEN p.nutriscore_grade = 'c' THEN 3
	    WHEN p.nutriscore_grade = 'd' THEN 4
	    WHEN p.nutriscore_grade = 'e' THEN 5
	    ELSE 6
	  END ASC
	LIMIT ?;`, mainClause)

	likePattern := "%" + primaryID + "%"
	rows, err := s.db.Query(sqlQuery, primaryID, likePattern, minPurity, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []model.CommercialProductWithBase
	for rows.Next() {
		var item model.CommercialProductWithBase
		var brands, nutriscore, nova, ingr, cat, img sql.NullString
		if err := rows.Scan(
			&item.Product.Code, &item.Product.ProductName, &brands, &nutriscore, &nova,
			&ingr, &cat, &img,
			&item.Product.EnergyKcal, &item.Product.Protein, &item.Product.Fat, &item.Product.Carbs, &item.Product.Fiber, &item.Product.Salt,
			&item.BaseFoodID, &item.Position, &item.Percentage, &item.IsMainBase, &item.PurityScore,
		); err != nil {
			return nil, err
		}
		item.Product.Brands = brands.String
		item.Product.NutriscoreGrade = nutriscore.String
		item.Product.NovaGroup = nova.String
		item.Product.IngredientsText = ingr.String
		item.Product.CategoriesTags = cat.String
		item.Product.ImageURL = img.String
		results = append(results, item)
	}

	return results, nil
}

// GetAllCommercialProductsWithIngredients devuelve todos los productos comerciales que tienen texto de ingredientes
func (s *Storage) GetAllCommercialProductsWithIngredients() ([]model.CommercialProduct, error) {
	rows, err := s.db.Query(`SELECT code, product_name, brands, nutriscore_grade, nova_group,
	       ingredients_text, categories_tags, image_url,
	       energy_kcal, protein, fat, carbs, fiber, salt
	FROM commercial_products
	WHERE ingredients_text != '' AND ingredients_text IS NOT NULL`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var prods []model.CommercialProduct
	for rows.Next() {
		var p model.CommercialProduct
		var brands, nutriscore, nova, ingr, cat, img sql.NullString
		if err := rows.Scan(
			&p.Code, &p.ProductName, &brands, &nutriscore, &nova,
			&ingr, &cat, &img,
			&p.EnergyKcal, &p.Protein, &p.Fat, &p.Carbs, &p.Fiber, &p.Salt,
		); err != nil {
			return nil, err
		}
		p.Brands = brands.String
		p.NutriscoreGrade = nutriscore.String
		p.NovaGroup = nova.String
		p.IngredientsText = ingr.String
		p.CategoriesTags = cat.String
		p.ImageURL = img.String
		prods = append(prods, p)
	}
	return prods, nil
}

// CountIndexedBases devuelve el número de enlaces en product_primary_bases
func (s *Storage) CountIndexedBases() (int, error) {
	var count int
	err := s.db.QueryRow("SELECT COUNT(*) FROM product_primary_bases").Scan(&count)
	return count, err
}


