package seed

import (
	_ "embed"
	"encoding/json"
	"fmt"

	"github.com/josejuanmontiel/primaryfoods/internal/model"
)

//go:embed foods.json
var defaultFoodsJSON []byte

// GetDefaultFoods devuelve la lista de alimentos semilla precargados
func GetDefaultFoods() ([]model.PrimaryFood, error) {
	var foods []model.PrimaryFood
	if err := json.Unmarshal(defaultFoodsJSON, &foods); err != nil {
		return nil, fmt.Errorf("error deserializando foods.json embebido: %w", err)
	}
	return foods, nil
}
