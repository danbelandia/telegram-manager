// Package api expone la API REST HTTP del backend.
package api

import (
	"encoding/json"
	"net/http"
)

// envelope es la forma de respuesta JSON consistente del API:
// {"data": ..., "error": null} o {"data": null, "error": {...}}.
// Los codigos de error siguen la seccion 18 del spec.
type envelope struct {
	Data  any       `json:"data"`
	Error *apiError `json:"error"`
}

type apiError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func writeJSON(w http.ResponseWriter, status int, body envelope) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func respond(w http.ResponseWriter, status int, data any) {
	writeJSON(w, status, envelope{Data: data, Error: nil})
}

func respondError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, envelope{Data: nil, Error: &apiError{Code: code, Message: message}})
}
