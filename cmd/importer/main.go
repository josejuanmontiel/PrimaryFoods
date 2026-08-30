package main

import (
	"flag"
	"fmt"
	"log"
	"time"

	"github.com/josejuanmontiel/primaryfoods/internal/importer"
	"github.com/josejuanmontiel/primaryfoods/internal/storage"
)

func main() {
	dbPath := flag.String("db", "data/primaryfoods.db", "Ruta de la base de datos SQLite")
	localCSV := flag.String("csv", "", "Ruta a un archivo CSV local de BEDCA (opcional, por defecto descarga online)")
	flag.Parse()

	log.Println("📥 Iniciando importación del dataset oficial BEDCA...")
	start := time.Now()

	foods, err := importer.ImportBEDCA(*localCSV)
	if err != nil {
		log.Fatalf("❌ Error procesando dataset BEDCA: %v", err)
	}

	log.Printf("📊 Parseados %d alimentos desde BEDCA en %v", len(foods), time.Since(start))

	store, err := storage.NewSQLiteStorage(*dbPath)
	if err != nil {
		log.Fatalf("❌ Error abriendo base de datos SQLite: %v", err)
	}
	defer store.Close()

	log.Println("💾 Guardando alimentos en la base de datos...")
	saved := 0
	for _, f := range foods {
		if err := store.Upsert(f); err != nil {
			log.Printf("⚠️  Error guardando '%s': %v", f.ID, err)
		} else {
			saved++
		}
	}

	log.Printf("🎉 Importación completada con éxito: %d alimentos guardados en %s (tiempo total: %v)", saved, *dbPath, time.Since(start))
	fmt.Println()
	fmt.Println("Ejemplos de alimentos importados:")
	for i, f := range foods {
		if i >= 5 {
			break
		}
		fmt.Printf("  • [%s] %s (%s) → %.0f kcal, %.1fg prot, %.1fg fibra\n", f.Category, f.Name, f.ID, f.NutritionPer100g.Calories, f.NutritionPer100g.Protein, f.NutritionPer100g.Fiber)
	}
}
