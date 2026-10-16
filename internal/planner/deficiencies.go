package planner

import (
	"fmt"
	"strings"
	"time"

	"github.com/josejuanmontiel/primaryfoods/internal/model"
)

// AnalyzeDiaryDeficiencies analiza las comidas consumidas en un rango de fechas o semana
// y detecta si se alcanzaron las metas mediterráneas (legumbres, pescado, pescado azul, vegetales).
func AnalyzeDiaryDeficiencies(
	entries []model.DiaryEntry,
	year int,
	week int,
	targetLegumes int,
	targetFish int,
) model.DiaryNutritionalSummary {
	if targetLegumes <= 0 {
		targetLegumes = 3
	}
	if targetFish <= 0 {
		targetFish = 2
	}
	targetOilyFish := 1 // Al menos 1 ración de pescado azul a la semana

	startDate, endDate := WeekDateRange(year, week)

	summary := model.DiaryNutritionalSummary{
		Year:         year,
		Week:         week,
		StartDate:    startDate,
		EndDate:      endDate,
		Deficiencies: make([]string, 0),
	}

	for _, entry := range entries {
		// Evaluamos comidas comprendidas en el rango
		if entry.Date < startDate || entry.Date > endDate {
			continue
		}

		// Si fue omitida
		if entry.Status == "skipped" {
			summary.SkippedMeals++
			continue
		}

		if !entry.AteAtHome {
			summary.EatenOutMeals++
		}

		// Solo contabilizamos nutricionalmente lo consumido
		hasLegume := false
		hasFish := false
		hasOilyFish := false
		hasVegetable := false
		isConsumed := entry.Status == "consumed" || entry.Status == ""

		for _, item := range entry.Items {
			if item.Status == "skipped" {
				continue
			}
			isConsumed = true

			text := strings.ToLower(item.Name)
			if item.Course == "drink" {
				continue
			}

			if isLegumeText(text) {
				hasLegume = true
			}
			if isFishText(text) {
				hasFish = true
				if isOilyFishText(text) {
					hasOilyFish = true
				}
			}
			if isVegetableText(text) {
				hasVegetable = true
			}
		}

		if isConsumed {
			summary.TotalConsumedMeals++
			if hasLegume {
				summary.LegumeMeals++
			}
			if hasFish {
				summary.FishMeals++
			}
			if hasOilyFish {
				summary.OilyFishMeals++
			}
			if hasVegetable {
				summary.VegetableMeals++
			}
		}
	}

	// Comprobación de carencias frente a metas mediterráneas
	if summary.LegumeMeals < targetLegumes {
		summary.LegumeDeficit = targetLegumes - summary.LegumeMeals
		summary.Deficiencies = append(summary.Deficiencies,
			fmt.Sprintf("Déficit de legumbres: se consumieron %d de las %d raciones objetivo (%d faltantes)",
				summary.LegumeMeals, targetLegumes, summary.LegumeDeficit))
	}

	if summary.FishMeals < targetFish {
		summary.FishDeficit = targetFish - summary.FishMeals
		summary.Deficiencies = append(summary.Deficiencies,
			fmt.Sprintf("Déficit de pescado: se consumieron %d de las %d raciones recomendadas (%d faltantes)",
				summary.FishMeals, targetFish, summary.FishDeficit))
	}

	if summary.OilyFishMeals < targetOilyFish {
		summary.OilyFishDeficit = targetOilyFish - summary.OilyFishMeals
		summary.Deficiencies = append(summary.Deficiencies,
			fmt.Sprintf("Falta de pescado azul (omega-3): %d raciones consumidas frente al mínimo de %d",
				summary.OilyFishMeals, targetOilyFish))
	}

	return summary
}

// WeekDateRange calcula la fecha inicial (lunes) y final (domingo) de una semana ISO
func WeekDateRange(year, week int) (string, string) {
	if year == 0 {
		year = time.Now().Year()
	}
	if week == 0 {
		_, week = time.Now().ISOWeek()
	}
	// 4 de enero siempre cae en la semana 1 ISO
	jan4 := time.Date(year, time.January, 4, 12, 0, 0, 0, time.UTC)
	weekday := int(jan4.Weekday())
	if weekday == 0 {
		weekday = 7 // Domingo es 7
	}
	// Lunes de la semana 1
	startWeek1 := jan4.AddDate(0, 0, -(weekday - 1))
	startOfWeek := startWeek1.AddDate(0, 0, (week-1)*7)
	endOfWeek := startOfWeek.AddDate(0, 0, 6)

	return startOfWeek.Format("2006-01-02"), endOfWeek.Format("2006-01-02")
}

func isLegumeText(text string) bool {
	return strings.Contains(text, "lenteja") || strings.Contains(text, "garbanzo") ||
		strings.Contains(text, "alubia") || strings.Contains(text, "judia") || strings.Contains(text, "legumbre")
}

func isFishText(text string) bool {
	return strings.Contains(text, "pescado") || strings.Contains(text, "merluza") ||
		strings.Contains(text, "bacalao") || strings.Contains(text, "salmon") ||
		strings.Contains(text, "sardina") || strings.Contains(text, "bonito") ||
		strings.Contains(text, "atun") || strings.Contains(text, "caballa") ||
		strings.Contains(text, "lubina") || strings.Contains(text, "dorada")
}

func isOilyFishText(text string) bool {
	return strings.Contains(text, "salmon") || strings.Contains(text, "sardina") ||
		strings.Contains(text, "bonito") || strings.Contains(text, "atun") ||
		strings.Contains(text, "caballa")
}

func isVegetableText(text string) bool {
	return strings.Contains(text, "ensalada") || strings.Contains(text, "verdura") ||
		strings.Contains(text, "espinaca") || strings.Contains(text, "acelga") ||
		strings.Contains(text, "brocoli") || strings.Contains(text, "calabacin") ||
		strings.Contains(text, "calabaza") || strings.Contains(text, "tomate")
}
