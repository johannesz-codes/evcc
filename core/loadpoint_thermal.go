package core

import (
	"errors"

	"github.com/evcc-io/evcc/api"
	"github.com/evcc-io/evcc/core/keys"
	"github.com/evcc-io/evcc/core/thermal"
)

// GetThermalFunctions returns independent, unmetered comfort functions.
func (lp *Loadpoint) GetThermalFunctions() []thermal.State {
	if lp.thermal == nil {
		return nil
	}
	return lp.thermal.Update()
}

// SetThermalBoost changes only the selected function's temporary comfort preload.
func (lp *Loadpoint) SetThermalBoost(kind string, enable bool) error {
	if lp.thermal == nil {
		return errors.New("thermal flexibility unavailable")
	}
	err := lp.thermal.SetBoost(kind, enable)
	lp.publishThermal()
	return err
}

// SetThermalBoostConfig changes the selected function's idle boost parameters.
func (lp *Loadpoint) SetThermalBoostConfig(kind string, config api.ThermalBoost) error {
	if lp.thermal == nil {
		return errors.New("thermal flexibility unavailable")
	}
	err := lp.thermal.SetBoostConfig(kind, config)
	lp.publishThermal()
	return err
}

func (lp *Loadpoint) publishThermal() {
	if lp.thermal != nil {
		lp.publish(keys.Thermal, lp.GetThermalFunctions())
	}
}
