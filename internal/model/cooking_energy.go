package model

// CookingEnergyClass clasifica el consumo de combustible de un alimento/receta
type CookingEnergyClass string

const (
	EnergyZero    CookingEnergyClass = "zero"    // 0 min de fuego (conservas, frutos secos, directo de envase)
	EnergyMinimal CookingEnergyClass = "minimal" // 1-3 min (calentar agua y reposo: cuscús, fideos finos, copos)
	EnergyMedium  CookingEnergyClass = "medium"  // 5-12 min (pasta fina, arroz blanco, lenteja roja, huevos)
	EnergyHigh    CookingEnergyClass = "high"    // 20-60+ min (legumbre seca, arroz integral, guisos lentos)
)

// CookingMethodInfo describe un método de preparación y sus ahorros
type CookingMethodInfo struct {
	Method             string             `json:"method"`             // ej: "passive_retention", "soak_and_boil", "direct_ready"
	ActiveFireMinutes  int                `json:"activeFireMinutes"`  // minutos de llama/fuego encendido
	PassiveWaitMinutes int                `json:"passiveWaitMinutes"` // minutos de reposo con calor residual
	EnergyClass        CookingEnergyClass `json:"energyClass"`        // zero, minimal, medium, high
	Tip                string             `json:"tip"`                // consejo culinario de ahorro
}

// FoodCookingProfile perfil energético de cocción de un alimento o ingrediente
type FoodCookingProfile struct {
	PrimaryFoodID        string              `json:"primaryFoodId"`
	FoodName             string              `json:"foodName"`
	Category             string              `json:"category"`
	DefaultEnergyClass   CookingEnergyClass  `json:"defaultEnergyClass"`
	DefaultFireMinutes   int                 `json:"defaultFireMinutes"`
	ZeroFuelAlternative string              `json:"zeroFuelAlternative,omitempty"` // ej: "garbanzo en conserva"
	EfficiencyTips       []string            `json:"efficiencyTips"`
	Methods              []CookingMethodInfo `json:"methods,omitempty"`
}
