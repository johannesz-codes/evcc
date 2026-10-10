package server

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/evcc-io/evcc/api"
	"github.com/evcc-io/evcc/core/loadpoint"
	"github.com/evcc-io/evcc/core/thermal"
	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

func TestThermalBoostHandler(t *testing.T) {
	for _, fail := range []bool{false, true} {
		lp := loadpoint.NewMockAPI(gomock.NewController(t))
		var err error
		if fail {
			err = errors.New("offline")
		} else {
			lp.EXPECT().GetThermalFunctions().Return([]thermal.State{{Kind: "spaceHeating", Requested: true, Active: true}})
		}
		lp.EXPECT().SetThermalBoost("spaceHeating", true).Return(err)
		r := mux.SetURLVars(httptest.NewRequest(http.MethodPost, "/", nil), map[string]string{"kind": "spaceHeating", "value": "true"})
		w := httptest.NewRecorder()
		thermalBoostHandler(lp)(w, r)
		if fail {
			assert.Equal(t, 400, w.Code)
		} else {
			assert.Equal(t, 200, w.Code)
			assert.Contains(t, w.Body.String(), `"active":true`)
			assert.NotContains(t, w.Body.String(), "power")
		}
	}
}

func TestThermalConfigHandler(t *testing.T) {
	for _, body := range []string{`{"delta":1.5}`, `{"mode":"large"}`, `{"delta":"wrong"}`, `{"unknown":1}`} {
		lp := loadpoint.NewMockAPI(gomock.NewController(t))
		ok := body == `{"delta":1.5}` || body == `{"mode":"large"}`
		if ok {
			config := api.ThermalBoost{Delta: 1.5}
			if strings.Contains(body, "mode") {
				config = api.ThermalBoost{Mode: "large"}
			}
			lp.EXPECT().SetThermalBoostConfig("spaceHeating", config).Return(nil)
			lp.EXPECT().GetThermalFunctions().Return(nil)
		}
		r := mux.SetURLVars(httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body)), map[string]string{"kind": "spaceHeating"})
		w := httptest.NewRecorder()
		thermalConfigHandler(lp)(w, r)
		if ok {
			assert.Equal(t, 200, w.Code)
		} else {
			assert.Equal(t, 400, w.Code)
		}
	}
}
