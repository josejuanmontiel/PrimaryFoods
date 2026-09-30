package planner

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/josejuanmontiel/primaryfoods/internal/matcher"
	"github.com/josejuanmontiel/primaryfoods/internal/model"
)

var spanishDays = []string{
	"Lunes", "Martes", "Miércoles", "Jueves", "Viernes", "Sábado", "Domingo",
}

// MenuPlanner genera propuestas de menú semanal equilibradas
type MenuPlanner struct {
	matcher *matcher.RecipeMatcher
}

// NewMenuPlanner crea una instancia de MenuPlanner
func NewMenuPlanner(m *matcher.RecipeMatcher) *MenuPlanner {
	return &MenuPlanner{matcher: m}
}

// ScoredRecipe evalúa el ajuste de una receta respecto a estacionalidad, caducidad y perfil mediterráneo
type ScoredRecipe struct {
	Recipe          model.Recipe
	Score           float64
	MatchPercentage float64
	UsedExpiring    []string
	SeasonalFoods   []string
	DietaryTags     []string
	IsLegume        bool
	IsFish          bool
	IsOilyFish      bool
	EnergyClass     model.CookingEnergyClass
	CookingTip      string
}

// GenerateWeeklyPlan genera un plan de 7 días (almuerzos y cenas) balanceado
func (p *MenuPlanner) GenerateWeeklyPlan(
	recipes []model.Recipe,
	pantry []model.PantryItem,
	seasonalItems []model.SeasonalItem,
	opts model.MenuGenerationOptions,
) (*model.WeeklyMenuPlan, error) {
	if len(recipes) == 0 {
		return nil, fmt.Errorf("no hay recetas disponibles para generar el menú")
	}

	if opts.Year == 0 {
		opts.Year = time.Now().Year()
	}
	if opts.Week == 0 {
		_, w := time.Now().ISOWeek()
		opts.Week = w
	}
	if opts.TargetLegumeDays == 0 {
		opts.TargetLegumeDays = 3
	}
	if opts.TargetFishDays == 0 {
		opts.TargetFishDays = 3
	}

	// 1. Mapeo de alimentos estacionales normalizados -> nivel de disponibilidad
	seasonalMap := make(map[string]model.SeasonalAvailability)
	for _, si := range seasonalItems {
		norm := matcher.Normalize(si.FoodName)
		if norm != "" {
			seasonalMap[norm] = si.Availability
		}
	}

	// 2. Mapeo de ingredientes próximos a caducar (< 15 días o class "critical"/"short")
	now := time.Now()
	expiringMap := make(map[string]bool)
	for _, it := range pantry {
		norm := matcher.Normalize(it.ProductName)
		if norm == "" {
			continue
		}
		if it.ShelfClass == "critical" || it.ShelfClass == "short" {
			expiringMap[norm] = true
		} else if it.ExpiryDate != nil && *it.ExpiryDate != "" {
			if t, err := time.Parse("2006-01-02", *it.ExpiryDate); err == nil {
				if t.Sub(now) <= 15*24*time.Hour {
					expiringMap[norm] = true
				}
			}
		}
	}

	// 3. Evaluar y puntuar cada receta
	var scoredList []ScoredRecipe
	for _, rec := range recipes {
		scored := p.scoreRecipe(rec, pantry, seasonalMap, expiringMap, opts.PrioritizePantry, opts.FuelSavingMode)
		scoredList = append(scoredList, scored)
	}

	// 4. Asignar slots para 7 días: Almuerzo y Cena
	month := int(float64(opts.Week-1)/4.33) + 1
	if month > 12 {
		month = 12
	}

	plan := &model.WeeklyMenuPlan{
		Year:       opts.Year,
		WeekNumber: opts.Week,
		Month:      month,
		Slots:      make([]model.DailyMealSlot, 0, 14),
	}

	usedSlugs := make(map[string]int) // Control de repetición
	legumeCount := 0
	fishCount := 0
	oilyFishCount := 0
	totalSeasonalMatches := 0
	totalPantrySavings := 0
	zeroFuelCount := 0
	minimalFuelCount := 0

	mealTypes := []string{"almuerzo", "cena"}

	// Fecha aproximada del primer día de la semana seleccionada
	startDate := time.Date(opts.Year, 1, 1, 12, 0, 0, 0, time.UTC).AddDate(0, 0, (opts.Week-1)*7)

	for dayIdx := 0; dayIdx < 7; dayIdx++ {
		slotDate := startDate.AddDate(0, 0, dayIdx).Format("2006-01-02")
		dayName := spanishDays[dayIdx]

		for _, mType := range mealTypes {
			bestIdx := -1
			bestScore := -9999.0

			for i, cand := range scoredList {
				// Penalizar fuertemente si ya se usó esta semana
				timesUsed := usedSlugs[cand.Recipe.Slug]
				if timesUsed >= 1 {
					continue // Evitar repetir receta en la misma semana si hay suficientes
				}

				score := cand.Score

				// Control mediterráneo: incentivar legumbres si estamos por debajo del objetivo
				if cand.IsLegume && legumeCount < opts.TargetLegumeDays && mType == "almuerzo" {
					score += 25.0
				}
				// Incentivar pescado
				if cand.IsFish && fishCount < opts.TargetFishDays {
					score += 20.0
					if cand.IsOilyFish {
						score += 10.0
					}
				}

				// Almuerzos suelen admitir platos más densos (legumbres/guisos), cenas platos más ligeros
				if mType == "cena" && cand.IsLegume {
					score -= 10.0
				}

				if score > bestScore {
					bestScore = score
					bestIdx = i
				}
			}

			// Fallback: si todas se han usado, elegir la menos repetida
			if bestIdx == -1 {
				minUsed := 999
				for i, cand := range scoredList {
					if usedSlugs[cand.Recipe.Slug] < minUsed {
						minUsed = usedSlugs[cand.Recipe.Slug]
						bestIdx = i
					}
				}
			}

			chosen := scoredList[bestIdx]
			usedSlugs[chosen.Recipe.Slug]++

			if chosen.IsLegume {
				legumeCount++
			}
			if chosen.IsFish {
				fishCount++
				if chosen.IsOilyFish {
					oilyFishCount++
				}
			}
			if len(chosen.SeasonalFoods) > 0 {
				totalSeasonalMatches++
			}
			totalPantrySavings += len(chosen.UsedExpiring)

			if chosen.EnergyClass == model.EnergyZero {
				zeroFuelCount++
			} else if chosen.EnergyClass == model.EnergyMinimal {
				minimalFuelCount++
			}

			plan.Slots = append(plan.Slots, model.DailyMealSlot{
				Date:            slotDate,
				DayOfWeek:       dayName,
				MealType:        mType,
				Recipe:          chosen.Recipe,
				MatchPercentage: chosen.MatchPercentage,
				UsedExpiring:    chosen.UsedExpiring,
				SeasonalFoods:   chosen.SeasonalFoods,
				DietaryTags:     chosen.DietaryTags,
				EnergyClass:     chosen.EnergyClass,
				CookingTip:      chosen.CookingTip,
				Score:           bestScore,
			})
		}
	}

	plan.TotalMeals = len(plan.Slots)
	plan.LegumeDays = legumeCount
	plan.FishDays = fishCount
	plan.OilyFishDays = oilyFishCount
	plan.SeasonalMatches = totalSeasonalMatches
	plan.PantrySavings = totalPantrySavings
	plan.ZeroFuelMeals = zeroFuelCount
	plan.MinimalFuelMeals = minimalFuelCount

	return plan, nil
}

func (p *MenuPlanner) scoreRecipe(
	rec model.Recipe,
	pantry []model.PantryItem,
	seasonalMap map[string]model.SeasonalAvailability,
	expiringMap map[string]bool,
	prioritizePantry bool,
	optsFuelSaving bool,
) ScoredRecipe {
	sr := ScoredRecipe{
		Recipe: rec,
		Score:  50.0, // Base score
	}

	// 1. Detección de tags dietéticos y perfil
	lowerName := strings.ToLower(rec.Name)
	var fullText strings.Builder
	fullText.WriteString(lowerName)
	for _, ing := range rec.Ingredients {
		fullText.WriteString(" " + strings.ToLower(ing.Name))
	}
	text := fullText.String()

	if strings.Contains(text, "lenteja") || strings.Contains(text, "garbanzo") ||
		strings.Contains(text, "alubia") || strings.Contains(text, "judia") || strings.Contains(text, "legumbre") {
		sr.IsLegume = true
		sr.DietaryTags = append(sr.DietaryTags, "legumbre")
		sr.Score += 15.0
	}

	if strings.Contains(text, "pescado") || strings.Contains(text, "merluza") ||
		strings.Contains(text, "bacalao") || strings.Contains(text, "salmon") ||
		strings.Contains(text, "sardina") || strings.Contains(text, "bonito") ||
		strings.Contains(text, "atun") || strings.Contains(text, "caballa") {
		sr.IsFish = true
		sr.DietaryTags = append(sr.DietaryTags, "pescado")
		sr.Score += 15.0

		if strings.Contains(text, "salmon") || strings.Contains(text, "sardina") ||
			strings.Contains(text, "bonito") || strings.Contains(text, "atun") || strings.Contains(text, "caballa") {
			sr.IsOilyFish = true
			sr.DietaryTags = append(sr.DietaryTags, "pescado_azul")
			sr.Score += 10.0
		}
	}

	// 2. Coincidencia con despensa y caducidad
	if p.matcher != nil {
		matches := p.matcher.MatchRecipes([]model.Recipe{rec}, pantry, matcher.MatchOptions{})
		if len(matches) > 0 {
			sr.MatchPercentage = matches[0].MatchPercentage
			sr.Score += sr.MatchPercentage * 0.3 // Hasta +30 puntos por disponibilidad
		}
	}

	// Verificar ingredientes que caducan
	for _, ing := range rec.Ingredients {
		normIng := matcher.Normalize(ing.Name)
		for expItem := range expiringMap {
			if strings.Contains(normIng, expItem) || strings.Contains(expItem, normIng) {
				sr.UsedExpiring = append(sr.UsedExpiring, ing.Name)
				if prioritizePantry {
					sr.Score += 25.0 // Fuerte bonus por evitar desperdicio
				} else {
					sr.Score += 12.0
				}
				break
			}
		}
	}

	// 3. Coincidencia con ingredientes de temporada
	for _, ing := range rec.Ingredients {
		normIng := matcher.Normalize(ing.Name)
		for sFood, avail := range seasonalMap {
			if strings.Contains(normIng, sFood) || strings.Contains(sFood, normIng) {
				sr.SeasonalFoods = append(sr.SeasonalFoods, ing.Name)
				if avail == model.AvailabilityPeak {
					sr.Score += 20.0 // Peak season bonus
				} else {
					sr.Score += 10.0 // Normal season bonus
				}
				break
			}
		}
	}

	// 4. Clasificación de consumo energético / tiempo de fuego
	sr.EnergyClass = model.EnergyMedium // Default
	if strings.Contains(text, "lata") || strings.Contains(text, "bote") || strings.Contains(text, "conserva") ||
		strings.Contains(text, "ensalada") || strings.Contains(text, "hummus") || strings.Contains(text, "yogur") ||
		strings.Contains(text, "frutos secos") {
		sr.EnergyClass = model.EnergyZero
		sr.CookingTip = "Consumo directo sin fuego (0 min)"
	} else if strings.Contains(text, "cuscus") || strings.Contains(text, "pure de patata") || strings.Contains(text, "avena") ||
		strings.Contains(text, "huevo") || strings.Contains(text, "fideos de arroz") {
		sr.EnergyClass = model.EnergyMinimal
		sr.CookingTip = "Solo calentar agua y reposar tapado (1-2 min fuego)"
	} else if strings.Contains(text, "legumbres secas") || strings.Contains(text, "arroz integral") ||
		strings.Contains(text, "estofado") || strings.Contains(text, "cocido") {
		sr.EnergyClass = model.EnergyHigh
		sr.CookingTip = "Cocción larga: imprescindible remojo o usar alternativa en conserva para ahorrar combustible"
	} else {
		sr.CookingTip = "Cocinar con tapa encajada para ahorrar hasta un 25% de energía"
	}

	// 5. Bonus / penalización si se activa Modo Ahorro de Combustible (FuelSavingMode)
	if optsFuelSaving {
		switch sr.EnergyClass {
		case model.EnergyZero:
			sr.Score += 40.0 // Gran prioridad a recetas sin fuego
		case model.EnergyMinimal:
			sr.Score += 25.0 // Prioridad a recetas de hervor rápido
		case model.EnergyMedium:
			sr.Score += 0.0
		case model.EnergyHigh:
			sr.Score -= 50.0 // Penalizar recetas de gasto masivo de gas
		}
	}

	// Eliminar duplicados en tags
	sort.Strings(sr.DietaryTags)
	sr.DietaryTags = uniqueStrings(sr.DietaryTags)

	return sr
}

func uniqueStrings(slice []string) []string {
	if len(slice) == 0 {
		return slice
	}
	res := []string{slice[0]}
	for i := 1; i < len(slice); i++ {
		if slice[i] != slice[i-1] {
			res = append(res, slice[i])
		}
	}
	return res
}

// GenerateAnnualPlan genera un plan de 52 semanas garantizando espaciado de repeticiones >= minDaysBetweenRepeats
func (p *MenuPlanner) GenerateAnnualPlan(
	recipes []model.Recipe,
	pantry []model.PantryItem,
	allSeasonalItems []model.SeasonalItem,
	opts model.AnnualPlanOptions,
) (*model.AnnualMenuPlan, error) {
	if len(recipes) == 0 {
		return nil, fmt.Errorf("no hay recetas disponibles para planificar el año")
	}

	if opts.Year == 0 {
		opts.Year = time.Now().Year()
	}
	if opts.MinDaysBetweenRepeats <= 0 {
		opts.MinDaysBetweenRepeats = 10
	}
	if opts.TargetLegumeDays == 0 {
		opts.TargetLegumeDays = 3
	}
	if opts.TargetFishDays == 0 {
		opts.TargetFishDays = 3
	}

	// Agrupar items estacionales por mes (1-12)
	seasonalByMonth := make(map[int][]model.SeasonalItem)
	for _, si := range allSeasonalItems {
		seasonalByMonth[si.Month] = append(seasonalByMonth[si.Month], si)
	}

	annualPlan := &model.AnnualMenuPlan{
		Year:                  opts.Year,
		TotalWeeks:            52,
		MinDaysBetweenRepeats: opts.MinDaysBetweenRepeats,
		GeneratedAt:           time.Now().Format(time.RFC3339),
		Weeks:                 make([]model.WeeklyMenuPlan, 0, 52),
	}

	// Mapa global de último día en que se sirvió cada receta: slug -> dayIndex (0 a 364)
	lastServedDay := make(map[string]int)
	for _, r := range recipes {
		lastServedDay[r.Slug] = -9999
	}

	dayIndex := 0

	for weekNum := 1; weekNum <= 52; weekNum++ {
		month := int(float64(weekNum-1)/4.33) + 1
		if month > 12 {
			month = 12
		}

		monthSeasonal := seasonalByMonth[month]

		// Pre-calcular puntuaciones de recetas para este mes
		seasonalMap := make(map[string]model.SeasonalAvailability)
		for _, si := range monthSeasonal {
			norm := matcher.Normalize(si.FoodName)
			if norm != "" {
				seasonalMap[norm] = si.Availability
			}
		}

		var scoredList []ScoredRecipe
		for _, rec := range recipes {
			scored := p.scoreRecipe(rec, pantry, seasonalMap, nil, opts.PrioritizePantry, opts.FuelSavingMode)
			scoredList = append(scoredList, scored)
		}

		weekPlan := model.WeeklyMenuPlan{
			Year:       opts.Year,
			WeekNumber: weekNum,
			Month:      month,
			Slots:      make([]model.DailyMealSlot, 0, 14),
		}

		legumeCount := 0
		fishCount := 0
		oilyFishCount := 0
		totalSeasonalMatches := 0
		zeroFuelCount := 0
		minimalFuelCount := 0

		startDate := time.Date(opts.Year, 1, 1, 12, 0, 0, 0, time.UTC).AddDate(0, 0, (weekNum-1)*7)

		for dayOffset := 0; dayOffset < 7; dayOffset++ {
			currentDayIdx := dayIndex + dayOffset
			slotDate := startDate.AddDate(0, 0, dayOffset).Format("2006-01-02")
			dayName := spanishDays[dayOffset]

			for _, mType := range []string{"almuerzo", "cena"} {
				bestIdx := -1
				bestScore := -9999.0

				for i, cand := range scoredList {
					daysSinceLast := currentDayIdx - lastServedDay[cand.Recipe.Slug]
					if daysSinceLast < opts.MinDaysBetweenRepeats {
						continue // Violación de regla de no repetición
					}

					score := cand.Score

					if cand.IsLegume && legumeCount < opts.TargetLegumeDays && mType == "almuerzo" {
						score += 25.0
					}
					if cand.IsFish && fishCount < opts.TargetFishDays {
						score += 20.0
						if cand.IsOilyFish {
							score += 10.0
						}
					}
					if mType == "cena" && cand.IsLegume {
						score -= 10.0
					}

					// Pequeño bono de frescura proporcional al tiempo sin servir
					score += float64(daysSinceLast) * 0.1

					if score > bestScore {
						bestScore = score
						bestIdx = i
					}
				}

				// Fallback: si ninguna cumple el intervalo estricto, elegir la servida hace más tiempo
				if bestIdx == -1 {
					oldestServed := 999999
					for i, cand := range scoredList {
						last := lastServedDay[cand.Recipe.Slug]
						if last < oldestServed {
							oldestServed = last
							bestIdx = i
						}
					}
				}

				chosen := scoredList[bestIdx]
				lastServedDay[chosen.Recipe.Slug] = currentDayIdx

				if chosen.IsLegume {
					legumeCount++
				}
				if chosen.IsFish {
					fishCount++
					if chosen.IsOilyFish {
						oilyFishCount++
					}
				}
				if len(chosen.SeasonalFoods) > 0 {
					totalSeasonalMatches++
				}
				if chosen.EnergyClass == model.EnergyZero {
					zeroFuelCount++
				} else if chosen.EnergyClass == model.EnergyMinimal {
					minimalFuelCount++
				}

				weekPlan.Slots = append(weekPlan.Slots, model.DailyMealSlot{
					Date:            slotDate,
					DayOfWeek:       dayName,
					MealType:        mType,
					Recipe:          chosen.Recipe,
					MatchPercentage: chosen.MatchPercentage,
					SeasonalFoods:   chosen.SeasonalFoods,
					DietaryTags:     chosen.DietaryTags,
					EnergyClass:     chosen.EnergyClass,
					CookingTip:      chosen.CookingTip,
					Score:           bestScore,
				})
			}
		}

		weekPlan.TotalMeals = len(weekPlan.Slots)
		weekPlan.LegumeDays = legumeCount
		weekPlan.FishDays = fishCount
		weekPlan.OilyFishDays = oilyFishCount
		weekPlan.SeasonalMatches = totalSeasonalMatches
		weekPlan.ZeroFuelMeals = zeroFuelCount
		weekPlan.MinimalFuelMeals = minimalFuelCount

		annualPlan.Weeks = append(annualPlan.Weeks, weekPlan)

		annualPlan.TotalMeals += weekPlan.TotalMeals
		annualPlan.TotalLegumeMeals += legumeCount
		annualPlan.TotalFishMeals += fishCount
		annualPlan.TotalOilyFishMeals += oilyFishCount
		annualPlan.ZeroFuelMeals += zeroFuelCount
		annualPlan.MinimalFuelMeals += minimalFuelCount

		dayIndex += 7
	}

	return annualPlan, nil
}

// ExportAnnualPlanToBuscatureceta transforma el plan anual a formato BuscaturecetaPackage (DiaryEntries)
func ExportAnnualPlanToBuscatureceta(plan *model.AnnualMenuPlan) *model.BuscaturecetaPackage {
	var entries []model.DiaryEntry
	var entryID int64 = 1

	for _, w := range plan.Weeks {
		for _, slot := range w.Slots {
			mealType := "lunch"
			if slot.MealType == "cena" {
				mealType = "dinner"
			}

			item := model.DiaryItem{
				Course:   "main",
				Type:     "recipe",
				Name:     slot.Recipe.Name,
				Servings: 1.0,
				Status:   "planned",
			}

			entries = append(entries, model.DiaryEntry{
				ID:        entryID,
				Date:      slot.Date,
				MealType:  mealType,
				Status:    "planned",
				AteAtHome: true,
				Version:   1,
				Items:     []model.DiaryItem{item},
			})
			entryID++
		}
	}

	return &model.BuscaturecetaPackage{
		SchemaVersion: "1.0.0",
		App:           "PrimaryFoods Annual Planner",
		ExportedAt:    time.Now().Format(time.RFC3339),
		Diary:         entries,
	}
}
