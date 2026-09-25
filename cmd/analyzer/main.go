package main

import (
	"flag"
	"fmt"
	"log"
	"strings"

	"github.com/josejuanmontiel/primaryfoods/internal/importer"
	"github.com/josejuanmontiel/primaryfoods/internal/matcher"
	"github.com/josejuanmontiel/primaryfoods/internal/storage"
)

func main() {
	dbPath := flag.String("db", "data/primaryfoods.db", "Ruta de la base de datos SQLite")
	recipesFile := flag.String("recipes", "data/qwen_recipes.json", "Ruta del archivo JSON de recetas")
	deconstructCode := flag.String("deconstruct", "", "Código EAN de un producto comercial para deconstruir sus ingredientes")
	byBaseFood := flag.String("by-base", "", "Alimento primario para buscar productos que lo contienen como base (ej: garbanzo, tomate, cacao)")
	flag.Parse()

	store, err := storage.NewSQLiteStorage(*dbPath)
	if err != nil {
		log.Fatalf("❌ Error abriendo SQLite: %v", err)
	}
	defer store.Close()

	foods, err := store.ExportAll()
	if err != nil {
		log.Fatalf("❌ Error cargando alimentos: %v", err)
	}

	labelParser := importer.NewLabelParser(foods)

	// Opción 1: Deconstruir un producto por código EAN
	if *deconstructCode != "" {
		prod, err := store.GetCommercialProductByCode(*deconstructCode)
		if err != nil || prod == nil {
			log.Fatalf("❌ Producto con código '%s' no encontrado", *deconstructCode)
		}
		decon := labelParser.DeconstructIngredients(*prod)
		fmt.Println("═══════════════════════════════════════════════════════════════════")
		fmt.Printf("🏷️  DECONSTRUCCIÓN NORMATIVA (REGLAMENTO UE 1169/2011)\n")
		fmt.Println("═══════════════════════════════════════════════════════════════════")
		fmt.Printf("Producto:         %s\n", decon.ProductName)
		fmt.Printf("Marca:            %s\n", decon.Brands)
		fmt.Printf("Código EAN:       %s\n", decon.ProductCode)
		fmt.Printf("Base Principal:   %s\n", decon.MainBase)
		fmt.Printf("Top Bases:        %v\n", decon.TopBases)
		fmt.Printf("Métrica Pureza:   %.1f / 100\n", decon.EstimatedPurityScore)
		fmt.Printf("Azúcar añadido:   %v\n", decon.HasAddedSugars)
		fmt.Printf("Grasa refinada:   %v\n", decon.HasRefinedOils)
		fmt.Printf("Aditivos totales: %d (Alto riesgo: %v)\n", decon.AdditivesCount, decon.HighRiskAdditives)
		fmt.Printf("\nIngredientes en orden de peso decreciente:\n")
		for _, ing := range decon.Ingredients {
			pct := ""
			if ing.Percentage != nil {
				pct = fmt.Sprintf(" [%.1f%%]", *ing.Percentage)
			}
			role := "🌿 Noble"
			if ing.IsAdditive {
				role = fmt.Sprintf("⚠️  Aditivo (%s | Riesgo %s)", ing.AdditiveCode, ing.AdditiveRisk)
			} else if ing.PrimaryFoodID != "" {
				role = fmt.Sprintf("🥦 Primario (%s)", ing.PrimaryFoodID)
			}
			fmt.Printf("  %2d. %-35s %-10s -> %s\n", ing.Position, ing.CleanName, pct, role)
		}
		return
	}

	// Opción 2: Buscador inverso por alimento primario base
	if *byBaseFood != "" {
		candidates, _ := store.SearchCommercialProducts(*byBaseFood, 40)
		fmt.Println("═══════════════════════════════════════════════════════════════════")
		fmt.Printf("🔍 PRODUCTOS COMERCIALES CON BASE PRIMARIA: '%s'\n", *byBaseFood)
		fmt.Println("═══════════════════════════════════════════════════════════════════")
		count := 0
		for _, p := range candidates {
			if p.IngredientsText == "" {
				continue
			}
			decon := labelParser.DeconstructIngredients(p)
			matches := strings.Contains(strings.ToLower(decon.MainBase), *byBaseFood)
			if !matches {
				for _, b := range decon.TopBases {
					if strings.Contains(strings.ToLower(b), *byBaseFood) {
						matches = true
						break
					}
				}
			}
			if matches {
				count++
				fmt.Printf("  %2d. [%s] %-35s (%s) | Pureza: %.0f | Nutri: %s | Nova: %s\n",
					count, p.Code, p.ProductName, p.Brands, decon.EstimatedPurityScore, stringsOr(p.NutriscoreGrade, "?"), stringsOr(p.NovaGroup, "?"))
				fmt.Printf("      Top Bases: %v | Aditivos: %d\n", decon.TopBases, decon.AdditivesCount)
			}
		}
		if count == 0 {
			fmt.Printf("No se encontraron productos cuya base sea '%s'\n", *byBaseFood)
		}
		return
	}

	recipes, err := importer.LoadRecipesFromJSONFile(*recipesFile)
	if err != nil {
		log.Fatalf("❌ Error cargando recetas: %v", err)
	}

	analyzer := matcher.NewRecipeAnalyzer(store, foods)

	totIng := 0
	totPrim := 0
	totComm := 0
	totUnres := 0

	fmt.Println("═══════════════════════════════════════════════════════════════════")
	fmt.Printf("🔍 INFORME DE ANÁLISIS DE RECETAS (PRIMARY FOODS + OPENFOODFACTS)\n")
	fmt.Println("═══════════════════════════════════════════════════════════════════")
	fmt.Println()

	for _, r := range recipes {
		analysis, err := analyzer.AnalyzeRecipe(r)
		if err != nil {
			log.Fatalf("Error analizando receta: %v", err)
		}

		statusIcon := "🟢"
		if analysis.MatchPercentage < 80 {
			statusIcon = "🟡"
		}
		if analysis.MatchPercentage < 50 {
			statusIcon = "🔴"
		}

		fmt.Printf("%s 🍽️  %s (Match: %.1f%% | %d/%d resueltos)\n", statusIcon, r.Name, analysis.MatchPercentage, analysis.PrimaryCount+analysis.CommercialCount, analysis.TotalIngredients)

		for _, res := range analysis.Resolutions {
			totIng++
			if res.Status == "primary_match" {
				totPrim++
				commInfo := ""
				if len(res.CommercialAlternatives) > 0 {
					p := res.CommercialAlternatives[0]
					brand := p.Brands
					if brand == "" {
						brand = "Genérico"
					}
					commInfo = fmt.Sprintf(" ➜ OFF: \"%s\" (%s | Nutri: %s | Nova: %s)", p.ProductName, brand, stringsOr(p.NutriscoreGrade, "?"), stringsOr(p.NovaGroup, "?"))
				}
				fmt.Printf("   🥦 [PRIMARIO] %-30s ➔ Base: %s%s\n", res.OriginalText, res.PrimaryFood.Name, commInfo)
			} else if res.Status == "commercial_match" {
				totComm++
				alt := ""
				if len(res.CommercialAlternatives) > 0 {
					p := res.CommercialAlternatives[0]
					brand := p.Brands
					if brand == "" {
						brand = "Genérico"
					}
					alt = fmt.Sprintf("➜ OFF: \"%s\" (%s | Nutri: %s | Nova: %s)", p.ProductName, brand, stringsOr(p.NutriscoreGrade, "?"), stringsOr(p.NovaGroup, "?"))
				}
				fmt.Printf("   🛒 [COMERCIAL] %-30s %s\n", res.OriginalText, alt)
			} else {
				totUnres++
				fmt.Printf("   ❌ [HUÉRFANO]  %-30s (limpio: '%s')\n", res.OriginalText, res.NormalizedName)
			}
		}
		fmt.Println()
	}

	fmt.Println("───────────────────────────────────────────────────────────────────")
	fmt.Printf("📊 RESUMEN GLOBAL:\n")
	fmt.Printf("   • Total Recetas:                %d\n", len(recipes))
	fmt.Printf("   • Total Ingredientes:           %d\n", totIng)
	fmt.Printf("   • Matches Alimento Primario:    %d (%.1f%%)\n", totPrim, float64(totPrim)/float64(totIng)*100)
	fmt.Printf("   • Matches Producto Comercial:   %d (%.1f%%)\n", totComm, float64(totComm)/float64(totIng)*100)
	fmt.Printf("   ─────────────────────────────────────────────────────────────\n")
	fmt.Printf("   🎉 COBERTURA TOTAL RESUELTA:    %d (%.1f%%)\n", totPrim+totComm, float64(totPrim+totComm)/float64(totIng)*100)
	fmt.Printf("   ⚠️  Huérfanos restantes:         %d (%.1f%%)\n", totUnres, float64(totUnres)/float64(totIng)*100)
	fmt.Println("═══════════════════════════════════════════════════════════════════")
}

func stringsOr(val, def string) string {
	if val == "" {
		return def
	}
	return val
}

func containsString(slice []string, val string) bool {
	for _, s := range slice {
		if s == val {
			return true
		}
	}
	return false
}

