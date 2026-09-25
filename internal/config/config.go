package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

// Config define los parámetros de configuración de PrimaryFoods
type Config struct {
	MealieURL   string `json:"mealieUrl"`
	MealieToken string `json:"mealieToken"`
	Port        string `json:"port,omitempty"`
	DBPath      string `json:"dbPath,omitempty"`
	ConfigPath  string `json:"-"`
}

// DefaultConfig devuelve una configuración con valores por defecto y detección de entorno
func DefaultConfig() *Config {
	cfg := &Config{
		MealieURL: "http://localhost:9925",
		Port:      "8086",
		DBPath:    "data/primaryfoods.db",
	}

	if u := os.Getenv("MEALIE_URL"); u != "" {
		cfg.MealieURL = u
	}
	if p := os.Getenv("PORT"); p != "" {
		cfg.Port = p
	}
	if d := os.Getenv("DB_PATH"); d != "" {
		cfg.DBPath = d
	}

	token := os.Getenv("MEALIE_TOKEN")
	if token == "" {
		token = os.Getenv("MEALIE_API_KEY")
	}
	if token == "" {
		token = detectMealieTokenFromEnvFiles()
	}
	cfg.MealieToken = token

	return cfg
}

// LoadConfig carga la configuración desde un archivo JSON o genera uno con valores detectados
func LoadConfig(path string) (*Config, error) {
	if path == "" {
		path = "config.json"
	}

	cfg := DefaultConfig()
	cfg.ConfigPath = path

	data, err := os.ReadFile(path)
	if err == nil {
		if err := json.Unmarshal(data, cfg); err != nil {
			return nil, err
		}
		// Rellenar campos faltantes si el fichero no los tenía
		if cfg.MealieURL == "" {
			cfg.MealieURL = "http://localhost:9925"
		}
		if cfg.Port == "" {
			cfg.Port = "8086"
		}
		if cfg.DBPath == "" {
			cfg.DBPath = "data/primaryfoods.db"
		}
		if cfg.MealieToken == "" {
			cfg.MealieToken = detectMealieTokenFromEnvFiles()
		}
		cfg.ConfigPath = path
		return cfg, nil
	}

	// Si no existe, guardar el archivo con los valores detectados
	_ = SaveConfig(path, cfg)
	return cfg, nil
}

// SaveConfig persiste la configuración en un archivo JSON formateado
func SaveConfig(path string, cfg *Config) error {
	if path == "" {
		if cfg.ConfigPath != "" {
			path = cfg.ConfigPath
		} else {
			path = "config.json"
		}
	}

	dir := filepath.Dir(path)
	if dir != "" && dir != "." {
		_ = os.MkdirAll(dir, 0755)
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

func detectMealieTokenFromEnvFiles() string {
	candidates := []string{
		"../sandbox/gemini/mealie/.env",
		"../../sandbox/gemini/mealie/.env",
		"/home/jose/workspace/sandbox/gemini/mealie/.env",
		".env",
	}

	for _, c := range candidates {
		content, err := os.ReadFile(c)
		if err != nil {
			continue
		}
		for _, line := range strings.Split(string(content), "\n") {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "MEALIE_API_KEY=") {
				return strings.TrimPrefix(line, "MEALIE_API_KEY=")
			}
			if strings.HasPrefix(line, "MEALIE_TOKEN=") {
				return strings.TrimPrefix(line, "MEALIE_TOKEN=")
			}
		}
	}
	return ""
}
