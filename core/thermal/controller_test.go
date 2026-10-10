package thermal

import (
	"encoding/json"
	"errors"
	"math"
	"testing"

	"github.com/evcc-io/evcc/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type testStore struct {
	values        map[string][]byte
	err           error
	saves, failAt int
}

func (s *testStore) Load(key string, dst any) error {
	if value := s.values[key]; value != nil {
		return json.Unmarshal(value, dst)
	}
	return nil
}
func (s *testStore) Save(key string, value any) error {
	s.saves++
	if s.saves == s.failAt {
		return errors.New("disk full")
	}
	if s.err != nil {
		return s.err
	}
	b, err := json.Marshal(value)
	s.values[key] = b
	return err
}

func TestEditableTargets(t *testing.T) {
	c, room, dhw, store := setup(t)
	require.NoError(t, c.SetTargets("spaceHeating", api.ThermalTargets{Normal: 20.7, Boost: 22.3}))
	assert.Equal(t, 20.7, room.value)
	assert.Equal(t, 1., dhw.value)
	assert.Equal(t, 1, room.writes)
	assert.InDelta(t, 22.3, *c.Update()[0].BoostTarget, 1e-6)
	require.NoError(t, c.SetBoost("spaceHeating", true))
	assert.InDelta(t, 22.3, room.value, 1e-6)
	require.NoError(t, c.SetTargets("spaceHeating", api.ThermalTargets{Normal: 21, Boost: 22.3}))
	assert.Equal(t, 2, room.writes, "editing normal during boost only changes the restoration value")
	require.NoError(t, c.SetTargets("spaceHeating", api.ThermalTargets{Normal: 21.7, Boost: 23.3}))
	room.value = float64(233) * room.spec.Step
	state := c.Update()[0]
	assert.True(t, state.Active, "register scaling must not lose ownership through floating point rounding")
	assert.Equal(t, 21.7, *state.NormalTarget)
	assert.InDelta(t, 23.3, *state.BoostTarget, 1e-6)
	require.NoError(t, c.SetBoost("spaceHeating", false))
	assert.Equal(t, 21.7, room.value)
	c, err := New(testDevice{room, dhw}, store)
	require.NoError(t, err)
	state = c.Update()[0]
	assert.Equal(t, 21.7, *state.NormalTarget)
	assert.InDelta(t, 23.3, *state.BoostTarget, 1e-6)
	room.value = 20
	state = c.Update()[0]
	assert.Equal(t, 20., *state.NormalTarget)
	assert.InDelta(t, 23.3, *state.BoostTarget, 1e-6, "absolute upper target must not drift with manual normal changes")
}

func TestTargetEditsPreserveExternalChanges(t *testing.T) {
	c, room, _, _ := setup(t)
	require.NoError(t, c.SetBoost("spaceHeating", true))
	room.value = 20
	assert.ErrorContains(t, c.SetTargets("spaceHeating", api.ThermalTargets{Normal: 21, Boost: 23}), "externally")
	assert.Equal(t, 20., room.value)
	assert.True(t, c.Update()[0].OwnershipLost)
	assert.Error(t, c.SetTargets("spaceHeating", api.ThermalTargets{Normal: 21, Boost: 23}))
	require.NoError(t, c.SetBoost("spaceHeating", false))
	require.NoError(t, c.SetTargets("spaceHeating", api.ThermalTargets{Normal: 21, Boost: 23}))
	require.NoError(t, c.SetBoost("spaceHeating", true))
	require.NoError(t, c.SetTargets("spaceHeating", api.ThermalTargets{Normal: 20.7, Boost: 23.3}))
	room.value = 23
	assert.True(t, c.Update()[0].OwnershipLost, "previous written value is released after an acknowledged edit")
	require.NoError(t, c.SetBoost("spaceHeating", false))
	assert.Equal(t, 23., room.value)
	assert.Equal(t, 4, room.writes)
}

func TestTargetValidation(t *testing.T) {
	c, room, _, _ := setup(t)
	for _, targets := range []api.ThermalTargets{
		{Normal: 4, Boost: 6}, {Normal: 29, Boost: 31}, {Normal: 21, Boost: 21},
		{Normal: 21, Boost: 20}, {Normal: 21, Boost: 26.1}, {Normal: 21.05, Boost: 22},
		{Normal: math.NaN(), Boost: 22}, {Normal: 21, Boost: math.Inf(1)},
	} {
		assert.Error(t, c.SetTargets("spaceHeating", targets))
	}
	assert.Error(t, c.SetTargets("domesticHotWater", api.ThermalTargets{Normal: 1, Boost: 2}))
	assert.Error(t, c.SetTargets("invalid", api.ThermalTargets{Normal: 21, Boost: 22}))
	assert.Equal(t, 0, room.writes)
	for _, boost := range []api.ThermalBoost{
		{Target: new(31.)}, {Target: new(22.), Delta: 1}, {Target: new(math.NaN())},
	} {
		assert.Error(t, c.SetBoostConfig("spaceHeating", boost))
	}
}

func TestAbsoluteBoostNeverLowersNormal(t *testing.T) {
	c, room, _, _ := setup(t)
	require.NoError(t, c.SetTargets("spaceHeating", api.ThermalTargets{Normal: 21, Boost: 22}))
	room.value = 23
	require.NoError(t, c.SetBoost("spaceHeating", true))
	assert.Equal(t, 23., room.value)
	assert.False(t, c.Update()[0].Active)
	require.NoError(t, c.SetBoost("spaceHeating", false))
	assert.Equal(t, 0, room.writes)
	room.value = 16
	assert.Error(t, c.SetBoost("spaceHeating", true), "external normal changes cannot bypass the 5 K limit")
	assert.Equal(t, 16., room.value)
}

func TestInterruptedNormalTargetEdit(t *testing.T) {
	for _, applied := range []bool{false, true} {
		t.Run(map[bool]string{false: "rejected", true: "lost acknowledgement"}[applied], func(t *testing.T) {
			c, room, dhw, store := setup(t)
			room.writeErr = errors.New("timeout")
			room.applyOnError = applied
			require.Error(t, c.SetTargets("spaceHeating", api.ThermalTargets{Normal: 20.7, Boost: 22.3}))
			room.readErr = errors.New("offline")
			c, err := New(testDevice{room, dhw}, store)
			require.NoError(t, err)
			assert.True(t, c.Update()[0].PendingTargetUpdate)
			room.readErr, room.writeErr = nil, nil
			state := c.Update()[0]
			assert.False(t, state.PendingTargetUpdate)
			assert.Equal(t, 1, room.writes, "recovery observes the applied write without repeating it")
			if applied {
				assert.Equal(t, 20.7, *state.NormalTarget)
				assert.InDelta(t, 22.3, *state.BoostTarget, 1e-6)
			} else {
				assert.Equal(t, 21., *state.NormalTarget)
				assert.Equal(t, 22., *state.BoostTarget)
			}
		})
	}
}

func TestPendingNormalEditPreservesExternalChange(t *testing.T) {
	c, room, dhw, store := setup(t)
	store.failAt = 2
	require.Error(t, c.SetTargets("spaceHeating", api.ThermalTargets{Normal: 20.7, Boost: 23.3}))
	room.value = 20
	c, err := New(testDevice{room, dhw}, store)
	require.NoError(t, err)
	state := c.Update()[0]
	assert.Equal(t, 20., room.value)
	assert.Equal(t, 1, room.writes)
	assert.False(t, state.PendingTargetUpdate)
	assert.Equal(t, 21., *state.BoostTarget, "an unconfirmed edit cannot commit its new boost target over a manual change")
}

func TestInterruptedActiveTargetEdit(t *testing.T) {
	for _, applied := range []bool{false, true} {
		t.Run(map[bool]string{false: "rejected", true: "lost acknowledgement"}[applied], func(t *testing.T) {
			c, room, dhw, store := setup(t)
			require.NoError(t, c.SetBoost("spaceHeating", true))
			room.writeErr = errors.New("timeout")
			room.applyOnError = applied
			require.Error(t, c.SetTargets("spaceHeating", api.ThermalTargets{Normal: 20.7, Boost: 23.3}))
			room.writeErr = nil
			c, err := New(testDevice{room, dhw}, store)
			require.NoError(t, err)
			state := c.Update()[0]
			assert.False(t, state.Requested)
			assert.False(t, state.PendingRestore)
			assert.Equal(t, 20.7, room.value, "either interrupted boost value restores the selected normal target")
			assert.InDelta(t, 23.3, *state.BoostTarget, 1e-6)
		})
	}
}

func TestTargetPersistenceFailures(t *testing.T) {
	for _, active := range []bool{false, true} {
		for _, failOffset := range []int{1, 2} {
			c, room, dhw, store := setup(t)
			if active {
				require.NoError(t, c.SetBoost("spaceHeating", true))
			}
			before := room.writes
			store.failAt = store.saves + failOffset
			require.Error(t, c.SetTargets("spaceHeating", api.ThermalTargets{Normal: 20.7, Boost: 23.3}))
			if failOffset == 1 {
				assert.Equal(t, before, room.writes, "journal failure must prevent a target write")
				continue
			}
			c, err := New(testDevice{room, dhw}, store)
			require.NoError(t, err)
			state := c.Update()[0]
			assert.Equal(t, 20.7, room.value)
			assert.InDelta(t, 23.3, *state.BoostTarget, 1e-6)
			assert.False(t, state.PendingRestore)
			assert.False(t, state.PendingTargetUpdate)
		}
	}
}

type testFunction struct {
	spec              api.ThermalSpec
	value             float64
	reads, writes     int
	readErr, writeErr error
	applyOnError      bool
}

func (f *testFunction) ThermalSpec() api.ThermalSpec { return f.spec }
func (f *testFunction) Target() (float64, error)     { f.reads++; return f.value, f.readErr }
func (f *testFunction) SetTarget(value float64) error {
	f.writes++
	if f.writeErr == nil || f.applyOnError {
		f.value = value
	}
	return f.writeErr
}

type testDevice []api.ThermalFunction

func (d testDevice) ThermalFunctions() []api.ThermalFunction { return d }

func setup(t *testing.T) (*Controller, *testFunction, *testFunction, *testStore) {
	t.Helper()
	room := &testFunction{value: 21, spec: api.ThermalSpec{Kind: "spaceHeating", Identity: "room", Min: 5, Max: 30, Step: .1, Boost: api.ThermalBoost{Delta: 1}}}
	dhw := &testFunction{value: 1, spec: api.ThermalSpec{Kind: "domesticHotWater", Identity: "dhw", Modes: map[string]float64{"small": 0, "medium": 1, "large": 2, "notInUse": 3, "smartControl": 4}, BoostModes: []string{"medium", "large"}, Boost: api.ThermalBoost{Mode: "large"}}}
	store := &testStore{values: make(map[string][]byte)}
	c, err := New(testDevice{room, dhw}, store)
	require.NoError(t, err)
	return c, room, dhw, store
}

func TestIndependentBoosts(t *testing.T) {
	c, room, dhw, _ := setup(t)
	require.NoError(t, c.SetBoost("spaceHeating", true))
	assert.Equal(t, 22., room.value)
	assert.Equal(t, 1., dhw.value)
	require.NoError(t, c.SetBoost("domesticHotWater", true))
	assert.Equal(t, 22., room.value)
	assert.Equal(t, 2., dhw.value)
	for range 5 {
		c.Update()
		require.NoError(t, c.SetBoost("spaceHeating", true))
	}
	assert.Equal(t, 1, room.writes, "repeated requests must not compound the delta")
	states := c.Update()
	assert.True(t, states[0].Active)
	assert.True(t, states[1].Active)
	require.NoError(t, c.SetBoost("spaceHeating", false))
	assert.Equal(t, 21., room.value)
	assert.Equal(t, 2., dhw.value)
	require.NoError(t, c.SetBoost("domesticHotWater", false))
	assert.Equal(t, 1., dhw.value)
	assert.Equal(t, 2, room.writes)
	assert.Equal(t, 2, dhw.writes)
	b, err := json.Marshal(states)
	require.NoError(t, err)
	assert.NotContains(t, string(b), "power")
	assert.NotContains(t, string(b), "energy")
}

func TestExternalChange(t *testing.T) {
	for _, kind := range []string{"spaceHeating", "domesticHotWater"} {
		t.Run(kind, func(t *testing.T) {
			c, room, dhw, _ := setup(t)
			f := room
			if kind == "domesticHotWater" {
				f = dhw
			}
			require.NoError(t, c.SetBoost(kind, true))
			f.value = 20
			if f == dhw {
				f.value = 4
			}
			value := f.value
			states := c.Update()
			index := 0
			if f == dhw {
				index = 1
			}
			assert.True(t, states[index].OwnershipLost)
			assert.False(t, states[index].Active)
			require.NoError(t, c.SetBoost(kind, true))
			assert.Equal(t, value, f.value)
			assert.Equal(t, 1, f.writes)
			require.NoError(t, c.SetBoost(kind, false))
			assert.Equal(t, value, f.value)
			require.NoError(t, c.SetBoost(kind, true))
			require.NoError(t, c.SetBoost(kind, false))
			assert.Equal(t, value, f.value, "new boost restores the new baseline")
		})
	}
}

func TestRestart(t *testing.T) {
	for _, external := range []bool{false, true} {
		t.Run(map[bool]string{false: "owned", true: "external"}[external], func(t *testing.T) {
			c, room, dhw, store := setup(t)
			require.NoError(t, c.SetBoost("spaceHeating", true))
			require.NoError(t, c.SetBoost("domesticHotWater", true))
			if external {
				room.value = 20
				dhw.value = 0
			}
			c, err := New(testDevice{room, dhw}, store)
			require.NoError(t, err)
			states := c.Update()
			for _, state := range states {
				assert.False(t, state.Requested)
				assert.False(t, state.Active)
				assert.False(t, state.PendingRestore)
			}
			if external {
				assert.Equal(t, 20., room.value)
				assert.Equal(t, 0., dhw.value)
			} else {
				assert.Equal(t, 21., room.value)
				assert.Equal(t, 1., dhw.value)
			}
		})
	}
}

func TestUnavailableRestore(t *testing.T) {
	c, room, dhw, store := setup(t)
	require.NoError(t, c.SetBoost("spaceHeating", true))
	room.readErr = errors.New("offline")
	require.Error(t, c.SetBoost("spaceHeating", false))
	state := c.Update()[0]
	assert.True(t, state.PendingRestore)
	assert.False(t, state.Available)
	assert.Equal(t, 22., room.value)
	c, err := New(testDevice{room, dhw}, store)
	require.NoError(t, err)
	assert.True(t, c.Update()[0].PendingRestore)
	room.readErr = nil
	room.writeErr = errors.New("write failed")
	assert.True(t, c.Update()[0].PendingRestore)
	room.writeErr = nil
	assert.False(t, c.Update()[0].PendingRestore)
	assert.Equal(t, 21., room.value)
}

func TestFailedEnable(t *testing.T) {
	for _, applied := range []bool{false, true} {
		t.Run(map[bool]string{false: "rejected", true: "lost acknowledgement"}[applied], func(t *testing.T) {
			c, room, _, _ := setup(t)
			room.writeErr = errors.New("timeout")
			room.applyOnError = applied
			require.Error(t, c.SetBoost("spaceHeating", true))
			assert.False(t, c.Update()[0].Requested)
			room.writeErr = nil
			c.Update()
			assert.Equal(t, 21., room.value)
			assert.Nil(t, c.functions[0].saved.Ownership)
		})
	}
}

func TestPersistenceFailure(t *testing.T) {
	c, room, _, store := setup(t)
	store.err = errors.New("disk full")
	require.Error(t, c.SetBoost("spaceHeating", true))
	assert.Equal(t, 0, room.writes)
	assert.Equal(t, 21., room.value)
}

func TestAlreadyAtBoostTarget(t *testing.T) {
	c, _, dhw, _ := setup(t)
	dhw.value = 2
	require.NoError(t, c.SetBoost("domesticHotWater", true))
	assert.Equal(t, 0, dhw.writes)
	assert.False(t, c.Update()[1].Active)
	dhw.value = 1
	c.Update()
	require.NoError(t, c.SetBoost("domesticHotWater", true))
	assert.Equal(t, 1., dhw.value)
	assert.Equal(t, 0, dhw.writes, "a satisfied request must not acquire a later user target")
	require.NoError(t, c.SetBoost("domesticHotWater", false))
	assert.Equal(t, 1., dhw.value)
}

func TestBoostValidation(t *testing.T) {
	c, room, _, _ := setup(t)
	for _, value := range []float64{0, -1, 6, .15, math.NaN(), math.Inf(1)} {
		assert.Error(t, c.SetBoostConfig("spaceHeating", api.ThermalBoost{Delta: value}))
	}
	assert.Error(t, c.SetBoostConfig("domesticHotWater", api.ThermalBoost{Mode: "notInUse"}))
	assert.Error(t, c.SetBoostConfig("domesticHotWater", api.ThermalBoost{Mode: "invalid"}))
	assert.Error(t, c.SetBoost("invalid", true))
	require.NoError(t, c.SetBoostConfig("spaceHeating", api.ThermalBoost{Delta: 1.5}))
	require.NoError(t, c.SetBoost("spaceHeating", true))
	assert.Equal(t, 22.5, room.value)
	assert.Error(t, c.SetBoostConfig("spaceHeating", api.ThermalBoost{Delta: 2}))
	require.NoError(t, c.SetBoost("spaceHeating", false))
	room.value = 30
	assert.Error(t, c.SetBoost("spaceHeating", true))
	assert.Equal(t, 30., room.value)
}

func TestChangedDeviceAndCorruptRecord(t *testing.T) {
	c, room, dhw, store := setup(t)
	require.NoError(t, c.SetBoost("spaceHeating", true))
	room.spec.Identity = "another device"
	_, err := New(testDevice{room, dhw}, store)
	require.ErrorContains(t, err, "device changed")
	store.values["spaceHeating"] = []byte(`{"identity":"another device","boost":{"delta":1},"ownership":{"baseline":0,"written":22}}`)
	_, err = New(testDevice{room, dhw}, store)
	require.ErrorContains(t, err, "invalid ownership")
	store.values["spaceHeating"] = []byte(`{broken`)
	_, err = New(testDevice{room, dhw}, store)
	require.Error(t, err)
}

func TestInvalidDeviceModes(t *testing.T) {
	_, room, dhw, store := setup(t)
	dhw.spec.Modes["medium"] = 2
	_, err := New(testDevice{room, dhw}, store)
	require.ErrorContains(t, err, "invalid comfort modes")
}
