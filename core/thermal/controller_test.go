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
	values map[string][]byte
	err    error
}

func (s *testStore) Load(key string, dst any) error {
	if value := s.values[key]; value != nil {
		return json.Unmarshal(value, dst)
	}
	return nil
}
func (s *testStore) Save(key string, value any) error {
	if s.err != nil {
		return s.err
	}
	b, err := json.Marshal(value)
	s.values[key] = b
	return err
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
