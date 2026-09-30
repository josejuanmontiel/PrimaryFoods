package seed

import (
	_ "embed"
	"encoding/json"
	"fmt"

	"github.com/josejuanmontiel/primaryfoods/internal/model"
)

//go:embed foods.json
var defaultFoodsJSON []byte

//go:embed seasonal_definitions.json
var defaultSeasonalDefinitionsJSON []byte

//go:embed cooking_energy.json
var defaultCookingEnergyJSON []byte

// SeasonalFoodDefinition estructura compacta para definir temporadas
type SeasonalFoodDefinition struct {
	PrimaryFoodID   string `json:"primaryFoodId"`
	FoodName        string `json:"foodName"`
	Category        string `json:"category"`
	PeakMonths      []int  `json:"peakMonths"`
	AvailableMonths []int  `json:"availableMonths"`
}

// GetDefaultFoods devuelve la lista de alimentos semilla precargados
func GetDefaultFoods() ([]model.PrimaryFood, error) {
	var foods []model.PrimaryFood
	if err := json.Unmarshal(defaultFoodsJSON, &foods); err != nil {
		return nil, fmt.Errorf("error deserializando foods.json embebido: %w", err)
	}
	return foods, nil
}

// GetDefaultSeasonalCalendar devuelve los datos de estacionalidad expandidos a 12 meses
func GetDefaultSeasonalCalendar() ([]model.SeasonalItem, error) {
	var defs []SeasonalFoodDefinition
	if err := json.Unmarshal(defaultSeasonalDefinitionsJSON, &defs); err != nil {
		return nil, fmt.Errorf("error deserializando seasonal_definitions.json: %w", err)
	}

	var items []model.SeasonalItem
	for _, def := range defs {
		for _, m := range def.PeakMonths {
			items = append(items, model.SeasonalItem{
				PrimaryFoodID: def.PrimaryFoodID,
				FoodName:      def.FoodName,
				Category:      def.Category,
				Month:         m,
				Availability:  model.AvailabilityPeak,
				Source:        "MAPA_ES",
			})
		}
		for _, m := range def.AvailableMonths {
			items = append(items, model.SeasonalItem{
				PrimaryFoodID: def.PrimaryFoodID,
				FoodName:      def.FoodName,
				Category:      def.Category,
				Month:         m,
				Availability:  model.AvailabilityAvailable,
				Source:        "MAPA_ES",
			})
		}
	}
	return items, nil
}

// GetDefaultCookingEnergyProfiles devuelve los perfiles de gasto energético y consejos
func GetDefaultCookingEnergyProfiles() ([]model.FoodCookingProfile, error) {
	var profiles []model.FoodCookingProfile
	if err := json.Unmarshal(defaultCookingEnergyJSON, &profiles); err != nil {
		return nil, fmt.Errorf("error deserializando cooking_energy.json: %w", err)
	}
	return profiles, nil
}
