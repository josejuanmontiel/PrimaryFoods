package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadAndSaveConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test_config.json")

	cfg, err := LoadConfig(configPath)
	if err != nil {
		t.Fatalf("LoadConfig falló: %v", err)
	}

	if cfg.MealieURL == "" {
		t.Errorf("MealieURL debería tener un valor por defecto")
	}

	cfg.MealieURL = "http://localhost:9999"
	cfg.MealieToken = "custom-token-123"

	if err := SaveConfig(configPath, cfg); err != nil {
		t.Fatalf("SaveConfig falló: %v", err)
	}

	// Recargar y verificar
	loaded, err := LoadConfig(configPath)
	if err != nil {
		t.Fatalf("Re-LoadConfig falló: %v", err)
	}

	if loaded.MealieURL != "http://localhost:9999" {
		t.Errorf("MealieURL esperado http://localhost:9999, obtenido %s", loaded.MealieURL)
	}
	if loaded.MealieToken != "custom-token-123" {
		t.Errorf("MealieToken esperado custom-token-123, obtenido %s", loaded.MealieToken)
	}

	_ = os.Remove(configPath)
}
