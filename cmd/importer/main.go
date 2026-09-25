package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/josejuanmontiel/primaryfoods/internal/importer"
	"github.com/josejuanmontiel/primaryfoods/internal/model"
	"github.com/josejuanmontiel/primaryfoods/internal/storage"
)

func main() {
	dbPath := flag.String("db", "data/primaryfoods.db", "Ruta de la base de datos SQLite")
	localCSV := flag.String("csv", "", "Ruta a un archivo CSV local de BEDCA (opcional)")
	doBedca := flag.Bool("bedca", false, "Ejecutar importación del dataset BEDCA")
	recipesFile := flag.String("recipes", "", "Ruta a un archivo JSON de recetas para importar")
	offFile := flag.String("off", "", "Ruta a spain_products.tsv.zz para importar productos comerciales")
	offLimit := flag.Int("off-limit", 50000, "Límite máximo de productos comerciales OFF a importar (0 para todos)")
	indexBases := flag.Bool("index-bases", false, "Deconstruir en lote e indexar bases primarias en SQLite")
	flag.Parse()

	store, err := storage.NewSQLiteStorage(*dbPath)
	if err != nil {
		log.Fatalf("❌ Error abriendo base de datos SQLite: %v", err)
	}
	defer store.Close()

	executedAny := false

	// 0. Indexar Bases Primarias si se solicitó
	if *indexBases {
		executedAny = true
		foods, err := store.ExportAll()
		if err != nil {
			log.Fatalf("❌ Error cargando alimentos: %v", err)
		}
		prodsCount, basesCount, err := importer.IndexProductBases(store, foods)
		if err != nil {
			log.Fatalf("❌ Error indexando bases: %v", err)
		}
		log.Printf("🎉 Indexación masiva completada: %d productos y %d bases relacionadas", prodsCount, basesCount)
	}

	// 1. Importar Recetas si se solicitó
	if *recipesFile != "" {
		executedAny = true
		log.Printf("📖 Importando recetas desde %s...", *recipesFile)
		start := time.Now()
		recipes, err := importer.LoadRecipesFromJSONFile(*recipesFile)
		if err != nil {
			log.Fatalf("❌ Error cargando recetas: %v", err)
		}
		if err := store.SaveRecipes(recipes); err != nil {
			log.Fatalf("❌ Error guardando recetas: %v", err)
		}
		log.Printf("🎉 %d recetas importadas con éxito en %v", len(recipes), time.Since(start))
		for _, r := range recipes {
			log.Printf("  • %s (%d ingredientes)", r.Name, len(r.Ingredients))
		}
	}

	// 2. Importar OpenFoodFacts si se solicitó
	if *offFile != "" {
		executedAny = true
		log.Printf("🛒 Importando productos comerciales de OpenFoodFacts desde %s (límite: %d)...", *offFile, *offLimit)
		start := time.Now()
		count, err := importer.ImportOFFProducts(importer.ImportOFFOptions{
			FilePath:            *offFile,
			MaxProducts:         *offLimit,
			OnlyWithIngredients: true,
			BatchSize:           2000,
		}, func(batch []model.CommercialProduct) error {
			return store.SaveCommercialProducts(batch)
		})
		if err != nil {
			log.Fatalf("❌ Error importando productos OFF: %v", err)
		}
		total, _ := store.CountCommercialProducts()
		log.Printf("🎉 %d productos OFF importados con éxito en %v (total en BD: %d)", count, time.Since(start), total)
	}

	// 3. Importar BEDCA si se solicitó explícitamente o si no se pasó ninguna otra opción
	if *doBedca || (!executedAny && *localCSV != "") {
		log.Println("📥 Iniciando importación del dataset oficial BEDCA...")
		start := time.Now()

		foods, err := importer.ImportBEDCA(*localCSV)
		if err != nil {
			log.Fatalf("❌ Error procesando dataset BEDCA: %v", err)
		}

		log.Printf("📊 Parseados %d alimentos desde BEDCA en %v", len(foods), time.Since(start))
		saved := 0
		for _, f := range foods {
			if err := store.Upsert(f); err != nil {
				log.Printf("⚠️  Error guardando '%s': %v", f.ID, err)
			} else {
				saved++
			}
		}
		log.Printf("🎉 %d alimentos guardados en %s (tiempo total: %v)", saved, *dbPath, time.Since(start))
	}

	if !executedAny && !*doBedca && *localCSV == "" {
		fmt.Println("Uso de primaryfoods-importer:")
		flag.PrintDefaults()
		os.Exit(0)
	}
}
