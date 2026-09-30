package importer

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/josejuanmontiel/primaryfoods/internal/model"
)

// ImportSeasonalFromSource lee datos de estacionalidad desde una URL remota o un fichero local
// Soporta formato JSON (lista de SeasonalItem o lista de SeasonalFoodDefinition) y CSV.
func ImportSeasonalFromSource(urlOrPath string) ([]model.SeasonalItem, error) {
	if strings.HasPrefix(urlOrPath, "http://") || strings.HasPrefix(urlOrPath, "https://") {
		resp, err := http.Get(urlOrPath)
		if err != nil {
			return nil, fmt.Errorf("error descargando calendario estacional desde %s: %w", urlOrPath, err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("código HTTP %d al descargar %s", resp.StatusCode, urlOrPath)
		}
		data, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, err
		}
		return parseSeasonalData(data)
	}

	f, err := os.Open(urlOrPath)
	if err != nil {
		return nil, fmt.Errorf("error abriendo archivo de estacionalidad local %s: %w", urlOrPath, err)
	}
	defer f.Close()

	data, err := io.ReadAll(f)
	if err != nil {
		return nil, err
	}
	return parseSeasonalData(data)
}

func parseSeasonalData(data []byte) ([]model.SeasonalItem, error) {
	// 1. Intentar como []model.SeasonalItem directo (debe tener Month especificado > 0)
	var directItems []model.SeasonalItem
	if err := json.Unmarshal(data, &directItems); err == nil && len(directItems) > 0 && directItems[0].FoodName != "" && directItems[0].Month > 0 {
		return directItems, nil
	}

	// 2. Intentar como definiciones compactas (peakMonths, availableMonths)
	type compactDef struct {
		PrimaryFoodID   string `json:"primaryFoodId"`
		FoodName        string `json:"foodName"`
		Category        string `json:"category"`
		PeakMonths      []int  `json:"peakMonths"`
		AvailableMonths []int  `json:"availableMonths"`
		Source          string `json:"source,omitempty"`
	}
	var compactDefs []compactDef
	if err := json.Unmarshal(data, &compactDefs); err == nil && len(compactDefs) > 0 {
		var res []model.SeasonalItem
		for _, d := range compactDefs {
			src := d.Source
			if src == "" {
				src = "REMOTE_IMPORT"
			}
			for _, m := range d.PeakMonths {
				res = append(res, model.SeasonalItem{
					PrimaryFoodID: d.PrimaryFoodID,
					FoodName:      d.FoodName,
					Category:      d.Category,
					Month:         m,
					Availability:  model.AvailabilityPeak,
					Source:        src,
				})
			}
			for _, m := range d.AvailableMonths {
				res = append(res, model.SeasonalItem{
					PrimaryFoodID: d.PrimaryFoodID,
					FoodName:      d.FoodName,
					Category:      d.Category,
					Month:         m,
					Availability:  model.AvailabilityAvailable,
					Source:        src,
				})
			}
		}
		return res, nil
	}

	// 3. Intentar como CSV (columnas: food_id, name, category, month, availability, source)
	csvReader := csv.NewReader(strings.NewReader(string(data)))
	records, err := csvReader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("no se pudo parsear el dataset como JSON ni como CSV: %w", err)
	}

	if len(records) < 2 {
		return nil, fmt.Errorf("dataset CSV vacío o sin cabecera")
	}

	header := records[0]
	colIdx := make(map[string]int)
	for i, h := range header {
		clean := strings.ToLower(strings.TrimSpace(strings.TrimPrefix(h, "\ufeff")))
		colIdx[clean] = i
	}

	var items []model.SeasonalItem
	for _, rec := range records[1:] {
		monthVal := 0
		if idx, ok := colIdx["month"]; ok && idx < len(rec) {
			monthVal, _ = strconv.Atoi(rec[idx])
		}
		if monthVal < 1 || monthVal > 12 {
			continue
		}

		name := ""
		if idx, ok := colIdx["name"]; ok && idx < len(rec) {
			name = rec[idx]
		} else if idx, ok := colIdx["food_name"]; ok && idx < len(rec) {
			name = rec[idx]
		}

		foodID := ""
		if idx, ok := colIdx["food_id"]; ok && idx < len(rec) {
			foodID = rec[idx]
		} else if idx, ok := colIdx["primary_food_id"]; ok && idx < len(rec) {
			foodID = rec[idx]
		}
		if foodID == "" && name != "" {
			foodID = strings.ToLower(strings.ReplaceAll(name, " ", "_"))
		}

		cat := "vegetable"
		if idx, ok := colIdx["category"]; ok && idx < len(rec) {
			cat = rec[idx]
		}

		avail := model.AvailabilityAvailable
		if idx, ok := colIdx["availability"]; ok && idx < len(rec) {
			if strings.EqualFold(rec[idx], "peak") {
				avail = model.AvailabilityPeak
			} else if strings.EqualFold(rec[idx], "low") {
				avail = model.AvailabilityLow
			}
		}

		src := "CSV_IMPORT"
		if idx, ok := colIdx["source"]; ok && idx < len(rec) && rec[idx] != "" {
			src = rec[idx]
		}

		items = append(items, model.SeasonalItem{
			PrimaryFoodID: foodID,
			FoodName:      name,
			Category:      cat,
			Month:         monthVal,
			Availability:  avail,
			Source:        src,
		})
	}

	return items, nil
}
