package measurement

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"math"

	"github.com/evcc-io/evcc/api"
	"github.com/evcc-io/evcc/plugin"
)

// Thermal configures optional direct comfort targets on a heating device.
type Thermal struct {
	Thermal map[string]ThermalConfig
}

// ThermalConfig uses the existing plugin transport for an unmetered function.
type ThermalConfig struct {
	Get, Set       plugin.Config
	Temperature    *plugin.Config
	Min, Max, Step float64
	Modes          map[string]float64
	BoostModes     []string
	Boost          api.ThermalBoost
}

type thermalDevice []api.ThermalFunction

var _ api.ThermalDevice = thermalDevice(nil)

func (d thermalDevice) ThermalFunctions() []api.ThermalFunction { return d }

type thermalFunction struct {
	spec api.ThermalSpec
	get  func() (float64, error)
	set  func(float64) error
}

var _ api.ThermalFunction = (*thermalFunction)(nil)

type thermalTemperature struct {
	*thermalFunction
	getTemperature func() (float64, error)
}

var _ api.ThermalTemperature = (*thermalTemperature)(nil)

func (f *thermalTemperature) Temperature() (float64, error) { return f.getTemperature() }

func (f *thermalFunction) ThermalSpec() api.ThermalSpec { return f.spec }
func (f *thermalFunction) Target() (float64, error)     { return f.get() }
func (f *thermalFunction) SetTarget(v float64) error {
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return fmt.Errorf("invalid thermal target: %v", v)
	}
	if len(f.spec.Modes) > 0 {
		for _, mode := range f.spec.Modes {
			if v == mode {
				return f.set(v)
			}
		}
		return fmt.Errorf("unsupported thermal mode: %v", v)
	}
	if v < f.spec.Min || v > f.spec.Max || math.Abs(v/f.spec.Step-math.Round(v/f.spec.Step)) > 1e-6 {
		return fmt.Errorf("thermal target must be %.1f to %.1f in %.1f steps", f.spec.Min, f.spec.Max, f.spec.Step)
	}
	return f.set(v)
}

// Configure creates a conditional capability without adding electrical consumers.
func (cc *Thermal) Configure(ctx context.Context) (api.ThermalDevice, error) {
	if len(cc.Thermal) == 0 {
		return nil, nil
	}
	var res thermalDevice
	for kind := range cc.Thermal {
		if kind != "spaceHeating" && kind != "domesticHotWater" {
			return nil, fmt.Errorf("unknown thermal function: %s", kind)
		}
	}
	for _, kind := range []string{"spaceHeating", "domesticHotWater"} {
		c, ok := cc.Thermal[kind]
		if !ok {
			continue
		}
		get, err := c.Get.FloatGetter(ctx)
		if err != nil {
			return nil, fmt.Errorf("thermal %s get: %w", kind, err)
		}
		if get == nil {
			return nil, fmt.Errorf("thermal %s: get is required", kind)
		}
		set, err := c.Set.FloatSetter(ctx, "target")
		if err != nil {
			return nil, fmt.Errorf("thermal %s set: %w", kind, err)
		}
		if set == nil {
			return nil, fmt.Errorf("thermal %s: set is required", kind)
		}
		identity, err := json.Marshal([]plugin.Config{c.Get, c.Set})
		if err != nil {
			return nil, err
		}
		temperature, err := c.Temperature.FloatGetter(ctx)
		if err != nil {
			return nil, fmt.Errorf("thermal %s temperature: %w", kind, err)
		}
		base := &thermalFunction{
			spec: api.ThermalSpec{Kind: kind, Identity: fmt.Sprintf("%x", sha256.Sum256(identity)), Min: c.Min, Max: c.Max, Step: c.Step, Modes: c.Modes, BoostModes: c.BoostModes, Boost: c.Boost},
			get:  get, set: set,
		}
		var function api.ThermalFunction = base
		if temperature != nil {
			function = &thermalTemperature{thermalFunction: base, getTemperature: temperature}
		}
		res = append(res, function)
	}
	return res, nil
}
