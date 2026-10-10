package server

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/evcc-io/evcc/api"
	"github.com/evcc-io/evcc/core/loadpoint"
	"github.com/gorilla/mux"
)

func thermalBoostHandler(lp loadpoint.API) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		value, err := strconv.ParseBool(vars["value"])
		if err == nil {
			err = lp.SetThermalBoost(vars["kind"], value)
		}
		if err != nil {
			jsonError(w, http.StatusBadRequest, err)
			return
		}
		jsonWrite(w, lp.GetThermalFunctions())
	}
}

func thermalConfigHandler(lp loadpoint.API) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var config api.ThermalBoost
		decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1024))
		decoder.DisallowUnknownFields()
		err := decoder.Decode(&config)
		if err == nil {
			err = lp.SetThermalBoostConfig(mux.Vars(r)["kind"], config)
		}
		if err != nil {
			jsonError(w, http.StatusBadRequest, err)
			return
		}
		jsonWrite(w, lp.GetThermalFunctions())
	}
}
