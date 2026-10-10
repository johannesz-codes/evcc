package charger

import (
	"testing"

	"github.com/evcc-io/evcc/api"
	"github.com/evcc-io/evcc/util/templates"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.yaml.in/yaml/v4"
)

func TestNibeThermalOptIn(t *testing.T) {
	for _, tc := range []struct {
		room, dhw bool
		functions int
	}{
		{false, false, 0}, {true, false, 1}, {false, true, 1}, {true, true, 2},
	} {
		tmpl, err := templates.ByName(templates.Charger, "nibe-s-series")
		require.NoError(t, err)
		rendered, _, err := tmpl.RenderResult(templates.Charger, templates.RenderModeInstance, map[string]any{"host": "127.0.0.1", "port": 5020, "roomcontrol": tc.room, "dhwcontrol": tc.dhw})
		require.NoError(t, err)
		var conf map[string]any
		require.NoError(t, yaml.Unmarshal(rendered, &conf))
		delete(conf, "type")
		device, err := NewSgReadyFromConfig(t.Context(), conf)
		require.NoError(t, err)
		thermal, ok := api.Cap[api.ThermalDevice](device)
		assert.Equal(t, tc.functions > 0, ok)
		if ok {
			assert.Len(t, thermal.ThermalFunctions(), tc.functions)
		}
		assert.Contains(t, string(rendered), "address: 6008")
		assert.Contains(t, string(rendered), "address: 6007")
		assert.Contains(t, string(rendered), "address: 2741")
		assert.NotContains(t, string(rendered), "address: 2740")
	}
}
