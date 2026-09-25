package importer

import (
	"bufio"
	"compress/gzip"
	"compress/zlib"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/josejuanmontiel/primaryfoods/internal/model"
)

// ImportOFFOptions opciones para la importación del volcado de OpenFoodFacts
type ImportOFFOptions struct {
	FilePath    string
	MaxProducts int  // 0 para sin límite
	OnlyWithIngredients bool // Si es true, solo importa productos con texto de ingredientes no vacío
	BatchSize   int
}

// ImportOFFProducts lee y procesa productos desde spain_products.tsv.zz o tsv plano en streaming
func ImportOFFProducts(opts ImportOFFOptions, onBatch func([]model.CommercialProduct) error) (int, error) {
	if opts.BatchSize <= 0 {
		opts.BatchSize = 5000
	}

	f, err := os.Open(opts.FilePath)
	if err != nil {
		return 0, fmt.Errorf("error abriendo archivo OFF: %w", err)
	}
	defer f.Close()

	var reader io.Reader = f
	if strings.HasSuffix(opts.FilePath, ".zz") {
		zr, err := zlib.NewReader(f)
		if err != nil {
			return 0, fmt.Errorf("error descomprimiendo zlib (.zz): %w", err)
		}
		defer zr.Close()
		reader = zr
	} else if strings.HasSuffix(opts.FilePath, ".gz") {
		gr, err := gzip.NewReader(f)
		if err != nil {
			return 0, fmt.Errorf("error descomprimiendo gzip (.gz): %w", err)
		}
		defer gr.Close()
		reader = gr
	}

	bufferedReader := bufio.NewReaderSize(reader, 1024*1024)

	// Leer cabecera TSV
	headerLine, err := bufferedReader.ReadString('\n')
	if err != nil {
		return 0, fmt.Errorf("error leyendo cabecera TSV: %w", err)
	}
	headerLine = strings.TrimRight(headerLine, "\r\n")
	headers := strings.Split(headerLine, "\t")
	col := make(map[string]int)
	for i, h := range headers {
		col[strings.TrimSpace(h)] = i
	}

	colCode, hasCode := col["code"]
	colName, hasName := col["product_name"]
	if !hasCode || !hasName {
		return 0, fmt.Errorf("formato no válido: columnas 'code' o 'product_name' no encontradas")
	}

	colBrands := getColIndex(col, "brands")
	colNutri := getColIndex(col, "nutriscore_grade")
	colNova := getColIndex(col, "nova_group")
	colIngr := getColIndex(col, "ingredients_text")
	colCat := getColIndex(col, "categories_tags")
	colImg := getColIndex(col, "image_url")
	colKcal := getColIndex(col, "energy-kcal_100g")
	colProt := getColIndex(col, "proteins_100g")
	colFat := getColIndex(col, "fat_100g")
	colCarb := getColIndex(col, "carbohydrates_100g")
	colFiber := getColIndex(col, "fiber_100g")
	colSalt := getColIndex(col, "salt_100g")

	var batch []model.CommercialProduct
	count := 0

	for {
		line, err := bufferedReader.ReadString('\n')
		if len(line) > 0 {
			line = strings.TrimRight(line, "\r\n")
			fields := strings.Split(line, "\t")

			if len(fields) > colName {
				name := strings.TrimSpace(cleanQuotes(fields[colName]))
				code := strings.TrimSpace(cleanQuotes(fields[colCode]))

				if name != "" && code != "" {
					ingrText := getField(fields, colIngr)
					if !opts.OnlyWithIngredients || ingrText != "" {
						nutri := strings.ToLower(getField(fields, colNutri))
						if nutri == "unknown" || nutri == "not-applicable" {
							nutri = ""
						}
						nova := getField(fields, colNova)
						if nova == "unknown" {
							nova = ""
						}

						prod := model.CommercialProduct{
							Code:            code,
							ProductName:     name,
							Brands:          getField(fields, colBrands),
							NutriscoreGrade: nutri,
							NovaGroup:       nova,
							IngredientsText: ingrText,
							CategoriesTags:  getField(fields, colCat),
							ImageURL:        getField(fields, colImg),
							EnergyKcal:      parseFloat(fields, colKcal),
							Protein:         parseFloat(fields, colProt),
							Fat:             parseFloat(fields, colFat),
							Carbs:           parseFloat(fields, colCarb),
							Fiber:           parseFloat(fields, colFiber),
							Salt:            parseFloat(fields, colSalt),
						}

						batch = append(batch, prod)
						count++

						if len(batch) >= opts.BatchSize {
							if err := onBatch(batch); err != nil {
								return count, err
							}
							batch = batch[:0]
						}

						if opts.MaxProducts > 0 && count >= opts.MaxProducts {
							break
						}
					}
				}
			}
		}

		if err != nil {
			if err == io.EOF {
				break
			}
			return count, err
		}
	}

	if len(batch) > 0 {
		if err := onBatch(batch); err != nil {
			return count, err
		}
	}

	return count, nil
}

func getColIndex(col map[string]int, key string) int {
	if idx, ok := col[key]; ok {
		return idx
	}
	return -1
}

func getField(fields []string, idx int) string {
	if idx >= 0 && idx < len(fields) {
		return cleanQuotes(strings.TrimSpace(fields[idx]))
	}
	return ""
}

func parseFloat(fields []string, idx int) float64 {
	if idx >= 0 && idx < len(fields) {
		val := cleanQuotes(strings.TrimSpace(fields[idx]))
		if val != "" {
			if f, err := strconv.ParseFloat(val, 64); err == nil {
				return f
			}
		}
	}
	return 0
}

func cleanQuotes(s string) string {
	return strings.Trim(s, "\"")
}
