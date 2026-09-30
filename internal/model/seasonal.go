package model

// SeasonalAvailability define el nivel de disponibilidad de un alimento en un mes/temporada
type SeasonalAvailability string

const (
	AvailabilityPeak SeasonalAvailability = "peak" // Temporada alta / óptima
	AvailabilityAvailable SeasonalAvailability = "available" // Disponible
	AvailabilityLow SeasonalAvailability = "low" // Escaso / fuera de temporada
)

// SeasonalFood representa la estacionalidad de un alimento para un mes o semana
type SeasonalItem struct {
	PrimaryFoodID string               `json:"primaryFoodId"`
	FoodName      string               `json:"foodName"`
	Category      string               `json:"category"`
	Month         int                  `json:"month"` // 1-12
	Availability  SeasonalAvailability `json:"availability"` // peak, available, low
	Source        string               `json:"source"` // ej: "MAPA_ES" (Ministerio Agricultura España)
}

// SeasonalCalendarFilter filtros para consultar estacionalidad
type SeasonalCalendarFilter struct {
	Month        int    `json:"month"`        // 1-12 (0 para todos)
	Category     string `json:"category"`     // "vegetable", "fruit", "fish"...
	Availability string `json:"availability"` // "peak", "available"
}
