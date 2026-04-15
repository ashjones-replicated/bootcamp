package handlers

import (
	"encoding/json"
	"net/http"
)

func (a *App) handleHealth(w http.ResponseWriter, r *http.Request) {
	type response struct {
		Status   string `json:"status"`
		Database string `json:"database"`
	}

	w.Header().Set("Content-Type", "application/json")

	if err := a.DB.Ping(r.Context()); err != nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(response{Status: "unhealthy", Database: "unavailable"})
		return
	}

	json.NewEncoder(w).Encode(response{Status: "ok", Database: "ok"})
}
