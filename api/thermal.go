package api

// ThermalBoost configures temporary comfort preload without disabling normal heating.
type ThermalBoost struct {
	Delta  float64  `json:"delta,omitempty"`
	Mode   string   `json:"mode,omitempty"`
	Target *float64 `json:"target,omitempty"`
}

// ThermalTargets sets the normal temperature and absolute boost temperature.
type ThermalTargets struct {
	Normal float64 `json:"normal"`
	Boost  float64 `json:"boost"`
}

// ThermalRange describes the supported temperature scale.
type ThermalRange struct {
	Min  float64 `json:"min"`
	Max  float64 `json:"max"`
	Step float64 `json:"step"`
}

// ThermalSpec describes an unmetered flexibility function of one physical device.
type ThermalSpec struct {
	Kind       string
	Identity   string
	Min        float64
	Max        float64
	Step       float64
	Modes      map[string]float64
	BoostModes []string
	Boost      ThermalBoost
}

// ThermalFunction provides direct comfort targets, independent of plant power control.
type ThermalFunction interface {
	ThermalSpec() ThermalSpec
	Target() (float64, error)
	SetTarget(float64) error
}

// ThermalTemperature optionally measures a function's current temperature.
type ThermalTemperature interface {
	Temperature() (float64, error)
}

// ThermalDevice exposes logical functions sharing its existing electrical meter.
type ThermalDevice interface {
	ThermalFunctions() []ThermalFunction
}
