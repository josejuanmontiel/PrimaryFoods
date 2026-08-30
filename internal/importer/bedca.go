package importer

import (
	"encoding/csv"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/josejuanmontiel/primaryfoods/internal/model"
)

const BEDCA_CSV_URL = "https://raw.githubusercontent.com/TerjeRu/bedca-database/master/output/bedca_foods.csv"

// ImportBEDCA descarga y parsea el dataset BEDCA transformándolo a []model.PrimaryFood
func ImportBEDCA(localFilePath string) ([]model.PrimaryFood, error) {
	var reader io.Reader

	if localFilePath != "" {
		f, err := os.Open(localFilePath)
		if err != nil {
			return nil, fmt.Errorf("error abriendo archivo local: %w", err)
		}
		defer f.Close()
		reader = f
	} else {
		resp, err := http.Get(BEDCA_CSV_URL)
		if err != nil {
			return nil, fmt.Errorf("error descargando dataset BEDCA: %w", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("HTTP error %d descargando BEDCA", resp.StatusCode)
		}
		reader = resp.Body
	}

	csvReader := csv.NewReader(reader)
	csvReader.LazyQuotes = true
	csvReader.TrimLeadingSpace = true

	// Leer cabecera
	header, err := csvReader.Read()
	if err != nil {
		return nil, fmt.Errorf("error leyendo cabecera CSV: %w", err)
	}

	colMap := make(map[string]int)
	for i, col := range header {
		// Quitar BOM si existe
		cleanCol := strings.TrimPrefix(col, "\ufeff")
		colMap[cleanCol] = i
	}

	var foods []model.PrimaryFood

	for {
		record, err := csvReader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			continue // saltar líneas con errores de formato
		}

		f := parseBEDCARecord(record, colMap)
		if f != nil && f.Name != "" {
			foods = append(foods, *f)
		}
	}

	return foods, nil
}

func parseBEDCARecord(rec []string, colMap map[string]int) *model.PrimaryFood {
	getVal := func(col string) string {
		idx, ok := colMap[col]
		if !ok || idx >= len(rec) {
			return ""
		}
		return strings.TrimSpace(rec[idx])
	}

	getFloat := func(col string) float64 {
		v := getVal(col)
		if v == "" {
			return 0
		}
		v = strings.ReplaceAll(v, ",", ".")
		f, _ := strconv.ParseFloat(v, 64)
		return f
	}

	id := getVal("f_id")
	name := getVal("f_ori_name")
	if name == "" {
		return nil
	}

	sciName := getVal("sci_name")
	engName := getVal("f_eng_name")

	// Energía en kJ a kcal (1 kcal = 4.184 kJ) o cálculo estándar
	enercKJ := getFloat("ENERC")
	enercUnit := strings.ToLower(getVal("ENERC_unit"))
	calories := enercKJ
	if enercUnit == "kj" && enercKJ > 0 {
		calories = enercKJ / 4.184
	}

	fat := getFloat("FAT")
	prot := getFloat("PROT")
	carbs := getFloat("CHO")
	fiber := getFloat("FIBT")
	sugars := getFloat("SUGAR")
	sodium := getFloat("NA")

	// Si calorías es 0 pero hay macros, calcular por Atwater: 4*prot + 4*carbs + 9*fat
	if calories == 0 && (fat > 0 || prot > 0 || carbs > 0) {
		calories = (prot * 4) + (carbs * 4) + (fat * 9)
	}

	category := inferCategory(name, getVal("namelevel1"), getVal("namelevel2"))
	aliases := generateAliases(name, engName)
	canonicalID := generateID(id, name)

	// Micronutrientes
	micros := make(map[string]float64)
	addMicro := func(key string, col string) {
		val := getFloat(col)
		if val > 0 {
			micros[key] = val
		}
	}

	addMicro("vitamina_c_mg", "VITC")
	addMicro("vitamina_a_ug", "VITA")
	addMicro("vitamina_d_ug", "VITD")
	addMicro("vitamina_e_mg", "VITE")
	addMicro("vitamina_b12_ug", "VITB12")
	addMicro("vitamina_b6_mg", "VITB6")
	addMicro("acido_folico_ug", "FOL")
	addMicro("calcio_mg", "CA")
	addMicro("hierro_mg", "FE")
	addMicro("magnesio_mg", "MG")
	addMicro("potasio_mg", "K")
	addMicro("fosforo_mg", "P")
	addMicro("selenio_ug", "SE")
	addMicro("zinc_mg", "ZN")

	tags, benefits := inferTagsAndBenefits(prot, fiber, fat, micros, category)

	return &model.PrimaryFood{
		ID:             canonicalID,
		Name:           cleanName(name),
		ScientificName: sciName,
		Category:       category,
		Aliases:        aliases,
		NutritionPer100g: model.Nutrition{
			Calories: roundFloat(calories, 1),
			Protein:  roundFloat(prot, 1),
			Fat:      roundFloat(fat, 1),
			Carbs:    roundFloat(carbs, 1),
			Fiber:    roundFloat(fiber, 1),
			Sugars:   roundFloat(sugars, 1),
			Sodium:   roundFloat(sodium, 1),
		},
		Micronutrients:     micros,
		BioactiveCompounds: []string{},
		Benefits:           benefits,
		Synergies:          []string{},
		Tags:               tags,
		DataSource:         "BEDCA",
	}
}

func inferCategory(name, level1, level2 string) string {
	combined := strings.ToLower(name + " " + level1 + " " + level2)

	switch {
	case containsAny(combined, "verdura", "hortaliz", "vegetal", "hoja", "espinaca", "col", "lechuga", "cebolla", "ajo", "tomate", "zanahoria", "calabac", "pimiento", "brocoli"):
		return "vegetable"
	case containsAny(combined, "fruta", "platano", "manzana", "pera", "naranja", "limon", "fresa", "arandano", "uva", "melon", "sandia", "melocoton"):
		return "fruit"
	case containsAny(combined, "legumbre", "lenteja", "garbanzo", "alubia", "judia", "soja", "guisante"):
		return "legume"
	case containsAny(combined, "cereal", "arroz", "trigo", "avena", "cebada", "centeno", "pan", "pasta", "harina", "maiz", "quinoa"):
		return "cereal"
	case containsAny(combined, "pescado", "marisco", "merluza", "salmon", "atun", "sardina", "bacalao", "gamba", "pulpo", "calamar"):
		return "fish"
	case containsAny(combined, "carne", "pollo", "pavo", "ternera", "cerdo", "buey", "conejo", "cordero"):
		return "meat"
	case containsAny(combined, "lacteo", "leche", "queso", "yogur", "mantequilla", "nata", "huevo"):
		return "dairy"
	case containsAny(combined, "fruto seco", "semilla", "nuez", "almendra", "avellana", "cacahuete", "pipa", "chia", "lino", "sesamo"):
		return "nut"
	case containsAny(combined, "aceite", "grasa", "oliva", "girasol"):
		return "oil"
	case containsAny(combined, "especia", "hierba", "sal", "pimienta", "oregano", "perejil", "laurel", "canela", "curcuma", "jengibre"):
		return "spice"
	default:
		return "other"
	}
}

func containsAny(text string, terms ...string) bool {
	for _, t := range terms {
		if strings.Contains(text, t) {
			return true
		}
	}
	return false
}

func cleanName(name string) string {
	// Poner en mayúscula la primera letra
	name = strings.TrimSpace(name)
	if len(name) > 0 {
		return strings.ToUpper(string(name[0])) + name[1:]
	}
	return name
}

func generateID(bedcaID, name string) string {
	clean := strings.ToLower(name)
	// Quitar acentos
	clean = removeAccents(clean)
	// Quitar caracteres especiales
	reg := regexp.MustCompile(`[^a-z0-9]+`)
	slug := reg.ReplaceAllString(clean, "_")
	slug = strings.Trim(slug, "_")
	if len(slug) > 40 {
		slug = slug[:40]
	}
	return fmt.Sprintf("bedca_%s_%s", bedcaID, slug)
}

func generateAliases(name, engName string) []string {
	seen := make(map[string]bool)
	var aliases []string

	add := func(s string) {
		s = strings.TrimSpace(strings.ToLower(s))
		if s != "" && !seen[s] {
			seen[s] = true
			aliases = append(aliases, s)
			// Versión sin acentos
			noAcc := removeAccents(s)
			if noAcc != s && !seen[noAcc] {
				seen[noAcc] = true
				aliases = append(aliases, noAcc)
			}
		}
	}

	add(name)
	if engName != "" {
		add(engName)
	}

	// Dividir por comas (ej: "Tomate, frito" -> "tomate frito", "tomate")
	parts := strings.Split(name, ",")
	if len(parts) > 1 {
		add(strings.Join(parts, " "))
		add(parts[0])
	}

	return aliases
}

func removeAccents(s string) string {
	replacer := strings.NewReplacer(
		"á", "a", "é", "e", "í", "i", "ó", "o", "ú", "u",
		"Á", "a", "É", "e", "Í", "i", "Ó", "o", "Ú", "u",
		"ñ", "n", "Ñ", "n", "ü", "u", "Ü", "u",
	)
	return replacer.Replace(s)
}

func inferTagsAndBenefits(prot, fiber, fat float64, micros map[string]float64, category string) ([]string, []string) {
	var tags []string
	var benefits []string

	if prot >= 10 {
		tags = append(tags, "alto-en-proteina")
		benefits = append(benefits, "Aporte significativo de proteínas para el mantenimiento muscular")
	}
	if fiber >= 5 {
		tags = append(tags, "rico-en-fibra")
		benefits = append(benefits, "Alto contenido en fibra dietética que apoya la salud digestiva y la microbiota")
	}
	if micros["vitamina_c_mg"] >= 30 {
		tags = append(tags, "vitamina-c", "antioxidante")
		benefits = append(benefits, "Rico en vitamina C para reforzar el sistema inmunológico")
	}
	if micros["hierro_mg"] >= 2.5 {
		tags = append(tags, "fuente-de-hierro")
		benefits = append(benefits, "Aporta hierro esencial para el transporte de oxígeno y combatir la fatiga")
	}
	if micros["calcio_mg"] >= 100 {
		tags = append(tags, "rico-en-calcio", "salud-osea")
		benefits = append(benefits, "Contribuye al mantenimiento de huesos y dientes en condiciones normales")
	}
	if micros["magnesio_mg"] >= 50 {
		tags = append(tags, "magnesio")
		benefits = append(benefits, "Fuente de magnesio que contribuye a la función muscular y reducción del cansancio")
	}
	if micros["vitamina_d_ug"] >= 2 {
		tags = append(tags, "vitamina-d")
		benefits = append(benefits, "Aporte de vitamina D esencial para la absorción de calcio y la inmunidad")
	}

	if category == "vegetable" || category == "fruit" {
		tags = append(tags, "alimento-fresco", "vegetal")
	}

	return tags, benefits
}

func roundFloat(val float64, precision int) float64 {
	p := 1.0
	for i := 0; i < precision; i++ {
		p *= 10
	}
	return float64(int(val*p+0.5)) / p
}
