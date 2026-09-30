package importer

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/josejuanmontiel/primaryfoods/internal/model"
)

// ImportCookingProfilesFromSource importa perfiles energéticos de cocción desde URL o archivo local
// Soporta el formato estándar de PrimaryFoods y perfiles de preparación (incluidos mapeos de FoodKeeper)
func ImportCookingProfilesFromSource(urlOrPath string) ([]model.FoodCookingProfile, error) {
	if strings.HasPrefix(urlOrPath, "http://") || strings.HasPrefix(urlOrPath, "https://") {
		resp, err := http.Get(urlOrPath)
		if err != nil {
			return nil, fmt.Errorf("error descargando perfiles de cocción desde %s: %w", urlOrPath, err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("código HTTP %d al descargar %s", resp.StatusCode, urlOrPath)
		}
		data, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, err
		}
		return parseCookingProfiles(data)
	}

	f, err := os.Open(urlOrPath)
	if err != nil {
		return nil, fmt.Errorf("error abriendo archivo de cocción local %s: %w", urlOrPath, err)
	}
	defer f.Close()

	data, err := io.ReadAll(f)
	if err != nil {
		return nil, err
	}
	return parseCookingProfiles(data)
}

func parseCookingProfiles(data []byte) ([]model.FoodCookingProfile, error) {
	// 1. Formato directo de PrimaryFoods
	var directProfiles []model.FoodCookingProfile
	if err := json.Unmarshal(data, &directProfiles); err == nil && len(directProfiles) > 0 && directProfiles[0].FoodName != "" {
		return directProfiles, nil
	}

	// 2. Formato FoodKeeper USDA (mapeo adaptativo de productos y guías de preparación)
	type foodKeeperWrapper struct {
		Sheets []struct {
			Name string `json:"name"`
			Data [][]interface{} `json:"data"`
		} `json:"sheets"`
	}
	var fk foodKeeperWrapper
	if err := json.Unmarshal(data, &fk); err == nil && len(fk.Sheets) > 0 {
		var profiles []model.FoodCookingProfile
		// Extraer datos si contiene hoja "Product"
		for _, s := range fk.Sheets {
			if strings.EqualFold(s.Name, "Product") || strings.EqualFold(s.Name, "Cooking") {
				for _, row := range s.Data {
					if len(row) >= 2 {
						name, ok1 := row[1].(string)
						if ok1 && name != "" {
							profiles = append(profiles, model.FoodCookingProfile{
								PrimaryFoodID:      strings.ToLower(strings.ReplaceAll(name, " ", "_")),
								FoodName:           name,
								Category:           "other",
								DefaultEnergyClass: model.EnergyMedium,
								DefaultFireMinutes: 10,
								EfficiencyTips:     []string{"Datos importados de USDA FoodKeeper"},
							})
						}
					}
				}
			}
		}
		if len(profiles) > 0 {
			return profiles, nil
		}
	}

	return nil, fmt.Errorf("formato no reconocido para perfiles de cocción")
}
