package importer

import (
	"fmt"
	"log"
	"time"

	"github.com/josejuanmontiel/primaryfoods/internal/model"
	"github.com/josejuanmontiel/primaryfoods/internal/storage"
)

// IndexProductBases procesa en lote todos los productos comerciales para poblar la tabla relacional product_primary_bases
func IndexProductBases(store *storage.Storage, foods []model.PrimaryFood) (int, int, error) {
	log.Println("🔍 Iniciando indexación masiva de bases primarias desde etiquetado legal...")
	start := time.Now()

	prods, err := store.GetAllCommercialProductsWithIngredients()
	if err != nil {
		return 0, 0, fmt.Errorf("error cargando productos: %w", err)
	}

	parser := NewLabelParser(foods)
	var batch []model.ProductPrimaryBase
	totalBases := 0
	batchSize := 2000

	for _, p := range prods {
		decon := parser.DeconstructIngredients(p)

		// Extraer cada ingrediente que tenga un alimento primario detectado
		seenForProduct := make(map[string]bool)

		for _, ing := range decon.Ingredients {
			if ing.PrimaryFoodID != "" && !seenForProduct[ing.PrimaryFoodID] {
				seenForProduct[ing.PrimaryFoodID] = true
				pct := 0.0
				if ing.Percentage != nil {
					pct = *ing.Percentage
				}
				isMain := ing.Position == 1

				batch = append(batch, model.ProductPrimaryBase{
					ProductCode:   p.Code,
					PrimaryFoodID: ing.PrimaryFoodID,
					Position:      ing.Position,
					Percentage:    pct,
					IsMainBase:    isMain,
					PurityScore:   decon.EstimatedPurityScore,
				})
				totalBases++

				if len(batch) >= batchSize {
					if err := store.SaveProductPrimaryBases(batch); err != nil {
						return len(prods), totalBases, fmt.Errorf("error guardando lote de bases: %w", err)
					}
					batch = batch[:0]
				}
			}
		}
	}

	if len(batch) > 0 {
		if err := store.SaveProductPrimaryBases(batch); err != nil {
			return len(prods), totalBases, fmt.Errorf("error guardando lote final de bases: %w", err)
		}
	}

	elapsed := time.Since(start)
	log.Printf("🎉 Indexación completada: %d productos analizados, %d enlaces de bases creados en %v", len(prods), totalBases, elapsed)
	return len(prods), totalBases, nil
}
