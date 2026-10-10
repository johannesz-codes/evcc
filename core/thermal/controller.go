// Package thermal manages temporary comfort preload on unmetered device functions.
package thermal

import (
	"errors"
	"fmt"
	"math"
	"slices"
	"sync"

	"github.com/evcc-io/evcc/api"
)

// Store saves ownership durably before writing a device target.
type Store interface {
	Load(string, any) error
	Save(string, any) error
}

// State is a logical function's UI/API state, without power or energy fields.
type State struct {
	Kind           string           `json:"kind"`
	Boost          api.ThermalBoost `json:"boost"`
	Requested      bool             `json:"requested"`
	Active         bool             `json:"active"`
	Available      bool             `json:"available"`
	OwnershipLost  bool             `json:"ownershipLost"`
	PendingRestore bool             `json:"pendingRestore"`
	Target         *float64         `json:"target,omitempty"`
	Mode           string           `json:"mode,omitempty"`
	Modes          []string         `json:"modes,omitempty"`
	BoostModes     []string         `json:"boostModes,omitempty"`
	Error          string           `json:"error,omitempty"`
}

type ownership struct {
	Baseline float64 `json:"baseline"`
	Written  float64 `json:"written"`
}

type saved struct {
	Identity  string           `json:"identity"`
	Boost     api.ThermalBoost `json:"boost"`
	Ownership *ownership       `json:"ownership,omitempty"`
}

type function struct {
	device                     api.ThermalFunction
	spec                       api.ThermalSpec
	saved                      saved
	requested, lost, satisfied bool
}

// Controller owns independent boost requests for functions of a single plant.
type Controller struct {
	mu        sync.Mutex
	store     Store
	functions []*function
}

// New restores ownership records. Previous boost requests are never resumed.
func New(device api.ThermalDevice, store Store) (*Controller, error) {
	c := &Controller{store: store}
	for _, dev := range device.ThermalFunctions() {
		spec := dev.ThermalSpec()
		values := make(map[float64]bool, len(spec.Modes))
		for mode, value := range spec.Modes {
			if mode == "" || !finite(value) || values[value] {
				return nil, fmt.Errorf("thermal %s: invalid comfort modes", spec.Kind)
			}
			values[value] = true
		}
		f := &function{device: dev, spec: spec, saved: saved{Identity: spec.Identity, Boost: spec.Boost}}
		if err := store.Load(spec.Kind, &f.saved); err != nil {
			return nil, err
		}
		if f.saved.Identity != spec.Identity {
			if f.saved.Ownership != nil {
				return nil, fmt.Errorf("thermal %s: device changed while restoration is pending", spec.Kind)
			}
			f.saved = saved{Identity: spec.Identity, Boost: spec.Boost}
		}
		if err := f.validateBoost(f.saved.Boost); err != nil {
			return nil, fmt.Errorf("thermal %s: %w", spec.Kind, err)
		}
		if len(spec.Modes) == 0 && (!finite(spec.Step) || !finite(spec.Min) || !finite(spec.Max) || spec.Step <= 0 || spec.Min >= spec.Max) {
			return nil, fmt.Errorf("thermal %s: invalid target range", spec.Kind)
		}
		if owned := f.saved.Ownership; owned != nil {
			if !f.validTarget(owned.Baseline) || !f.validTarget(owned.Written) {
				return nil, fmt.Errorf("thermal %s: invalid ownership record", spec.Kind)
			}
		}
		for _, existing := range c.functions {
			if existing.spec.Kind == spec.Kind {
				return nil, fmt.Errorf("duplicate thermal function: %s", spec.Kind)
			}
		}
		c.functions = append(c.functions, f)
	}
	return c, nil
}

func (f *function) validateBoost(b api.ThermalBoost) error {
	if len(f.spec.Modes) > 0 {
		if _, ok := f.spec.Modes[b.Mode]; !ok || b.Delta != 0 || !slices.Contains(f.spec.BoostModes, b.Mode) {
			return errors.New("unsupported boost mode")
		}
		return nil
	}
	if math.IsNaN(b.Delta) || math.IsInf(b.Delta, 0) || b.Delta <= 0 || b.Delta > 5 || b.Mode != "" || f.spec.Step <= 0 || math.Abs(b.Delta/f.spec.Step-math.Round(b.Delta/f.spec.Step)) > 1e-6 {
		return errors.New("boost delta must be positive, at most 5 K, and match target precision")
	}
	return nil
}

func finite(v float64) bool { return !math.IsNaN(v) && !math.IsInf(v, 0) }

func (f *function) validTarget(v float64) bool {
	if !finite(v) {
		return false
	}
	if len(f.spec.Modes) > 0 {
		for _, value := range f.spec.Modes {
			if v == value {
				return true
			}
		}
		return false
	}
	return v >= f.spec.Min && v <= f.spec.Max && math.Abs(v/f.spec.Step-math.Round(v/f.spec.Step)) < 1e-6
}

func (c *Controller) find(kind string) (*function, error) {
	for _, f := range c.functions {
		if f.spec.Kind == kind {
			return f, nil
		}
	}
	return nil, fmt.Errorf("thermal function unavailable: %s", kind)
}

// SetBoost adds or removes preload. It never switches normal heating off.
func (c *Controller) SetBoost(kind string, enable bool) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	f, err := c.find(kind)
	if err != nil {
		return err
	}
	if !enable {
		f.lost = false
	}
	if enable && !f.requested {
		if f.saved.Ownership != nil {
			if _, err := c.update(f); err != nil {
				return err
			}
		}
		f.requested = true
		f.satisfied = false
		if _, err := c.update(f); err != nil {
			f.requested = false
			return err
		}
		return nil
	}
	f.requested = enable
	_, err = c.update(f)
	return err
}

// SetBoostConfig changes preload parameters while the function is idle.
func (c *Controller) SetBoostConfig(kind string, boost api.ThermalBoost) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	f, err := c.find(kind)
	if err != nil {
		return err
	}
	if f.requested || f.saved.Ownership != nil {
		return errors.New("disable boost and finish restoration before changing its parameters")
	}
	if err := f.validateBoost(boost); err != nil {
		return err
	}
	next := f.saved
	next.Boost = boost
	if err := c.store.Save(kind, next); err != nil {
		return err
	}
	f.saved = next
	return nil
}

func (c *Controller) saveOwnership(f *function, owned *ownership) error {
	next := f.saved
	next.Ownership = owned
	if err := c.store.Save(f.spec.Kind, next); err != nil {
		return err
	}
	f.saved = next
	return nil
}

func (c *Controller) update(f *function) (State, error) {
	s := State{Kind: f.spec.Kind, Boost: f.saved.Boost, Requested: f.requested, OwnershipLost: f.lost, PendingRestore: !f.requested && f.saved.Ownership != nil, BoostModes: f.spec.BoostModes}
	for mode := range f.spec.Modes {
		s.Modes = append(s.Modes, mode)
	}
	slices.Sort(s.Modes)
	current, err := f.device.Target()
	if err != nil {
		return s, err
	}
	if !f.validTarget(current) {
		return s, errors.New("invalid thermal target")
	}
	s.Available = true
	if owned := f.saved.Ownership; owned != nil {
		if current != owned.Written {
			f.lost = f.requested
			if err := c.saveOwnership(f, nil); err != nil {
				return s, err
			}
		} else if !f.requested {
			if err := f.device.SetTarget(owned.Baseline); err != nil {
				return s, err
			}
			current = owned.Baseline
			if err := c.saveOwnership(f, nil); err != nil {
				return s, err
			}
		}
	}
	if f.requested && !f.lost && !f.satisfied && f.saved.Ownership == nil {
		boost := f.saved.Boost
		value := current + boost.Delta
		if len(f.spec.Modes) > 0 {
			value = f.spec.Modes[boost.Mode]
		} else {
			value = math.Round(value/f.spec.Step) * f.spec.Step
		}
		if len(f.spec.Modes) == 0 && (value < f.spec.Min || value > f.spec.Max) {
			return s, errors.New("boost target exceeds device comfort range")
		}
		if value != current {
			if err := c.saveOwnership(f, &ownership{Baseline: current, Written: value}); err != nil {
				return s, err
			}
			if err := f.device.SetTarget(value); err != nil {
				return s, err
			}
			current = value
		}
		f.satisfied = true
	}
	if len(f.spec.Modes) == 0 {
		s.Target = &current
	} else {
		for mode, value := range f.spec.Modes {
			if value == current {
				s.Mode = mode
			}
		}
		if s.Mode == "" {
			return s, errors.New("unknown device comfort mode")
		}
	}
	s.Requested, s.OwnershipLost = f.requested, f.lost
	s.PendingRestore = !f.requested && f.saved.Ownership != nil
	s.Active = f.saved.Ownership != nil && current == f.saved.Ownership.Written
	return s, nil
}

// Update reads targets and retries pending restoration after a connection failure.
func (c *Controller) Update() []State {
	c.mu.Lock()
	defer c.mu.Unlock()
	res := make([]State, 0, len(c.functions))
	for _, f := range c.functions {
		s, err := c.update(f)
		if err != nil {
			s.Error = err.Error()
			s.Available = false
		}
		res = append(res, s)
	}
	return res
}
