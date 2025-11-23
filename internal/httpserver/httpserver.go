package httpserver

import (
	"encoding/json"
	"fmt"
	"net/http"
)

type HandlerFunc func(http.ResponseWriter, *http.Request) error

type ErrorBody struct {
	Error string `json:"error"`
}

func WithError(next HandlerFunc) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				RespondError(w, http.StatusInternalServerError, fmt.Sprint(rec))
			}
		}()
		if err := next(w, r); err != nil {
			RespondError(w, http.StatusInternalServerError, err.Error())
		}
	})
}

func RespondJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func RespondError(w http.ResponseWriter, status int, msg string) {
	RespondJSON(w, status, ErrorBody{Error: msg})
}
