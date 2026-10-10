package server

import (
	"encoding/json"
	"errors"
	"io"
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

func thermalTargetsHandler(lp loadpoint.API) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var targets *api.ThermalTargets
		decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1024))
		decoder.DisallowUnknownFields()
		err := decoder.Decode(&targets)
		if err == nil && targets == nil {
			err = errors.New("temperature pair required")
		}
		if err == nil {
			var extra any
			if decoder.Decode(&extra) != io.EOF {
				err = errors.New("expected a single temperature pair")
			}
		}
		if err == nil {
			err = lp.SetThermalTargets(mux.Vars(r)["kind"], *targets)
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
