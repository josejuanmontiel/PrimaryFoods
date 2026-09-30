package shelflife

import (
	"testing"
	"time"
)

func TestEstimateShelfLife(t *testing.T) {
	baseDate := time.Date(2026, 9, 30, 0, 0, 0, 0, time.UTC)

	tests := []struct {
		name            string
		code            string
		pName           string
		tags            string
		primaryCat      string
		expectedDays    int
		expectedClass   ShelfClass
		expectedRel     string
	}{
		{
			name:          "Canned fish in OFF",
			code:          "8402001031076",
			pName:         "Caballa Del Sur En Aceite De Oliva",
			tags:          "en:mackerel-fillets-in-oil,en:tinned-mackerels",
			expectedDays:  1095,
			expectedClass: ClassLong,
			expectedRel:   "high",
		},
		{
			name:          "Dry pasta in OFF",
			code:          "8410069017956",
			pName:         "Margaritas con tomate",
			tags:          "en:plant-based-foods,en:pastas",
			expectedDays:  730,
			expectedClass: ClassLong,
			expectedRel:   "high",
		},
		{
			name:          "BEDCA fresh vegetable",
			code:          "primary:bedca_2399_lechuga",
			pName:         "Lechuga",
			primaryCat:    "vegetable",
			expectedDays:  7,
			expectedClass: ClassShort,
			expectedRel:   "high",
		},
		{
			name:          "Custom product keyword burger",
			code:          "GENERIC_8436548601706",
			pName:         "Burger Meat Vacuno",
			expectedDays:  5,
			expectedClass: ClassShort,
			expectedRel:   "medium",
		},
		{
			name:          "Unknown fallback",
			code:          "9999999999999",
			pName:         "Producto Desconocido XYZ",
			expectedDays:  60,
			expectedClass: ClassMedium,
			expectedRel:   "fallback",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := EstimateShelfLife(tt.code, tt.pName, tt.tags, tt.primaryCat, baseDate)
			if got.Days != tt.expectedDays {
				t.Errorf("EstimateShelfLife().Days = %v, want %v", got.Days, tt.expectedDays)
			}
			if got.ShelfClass != tt.expectedClass {
				t.Errorf("EstimateShelfLife().ShelfClass = %v, want %v", got.ShelfClass, tt.expectedClass)
			}
			if got.Reliability != tt.expectedRel {
				t.Errorf("EstimateShelfLife().Reliability = %v, want %v", got.Reliability, tt.expectedRel)
			}
		})
	}
}
