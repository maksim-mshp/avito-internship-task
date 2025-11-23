package httpserver

import (
	"encoding/json"
	"net/http"
)

type HandlerFunc func(http.ResponseWriter, *http.Request) error

type ErrorBody struct {
	Error string `json:"error"`
}

func WithError(next HandlerFunc) http.Handler {
	return Recover(next)
}

func RespondJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func RespondError(w http.ResponseWriter, status int, msg string) {
	RespondJSON(w, status, ErrorBody{Error: msg})
}
