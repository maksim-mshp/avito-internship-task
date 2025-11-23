package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
)

type appHandler func(http.ResponseWriter, *http.Request) error

type errorResponse struct {
	Error string `json:"error"`
}

func errorMiddleware(next appHandler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				respondError(w, http.StatusInternalServerError, fmt.Sprint(rec))
			}
		}()

		if err := next(w, r); err != nil {
			respondError(w, http.StatusInternalServerError, err.Error())
			return
		}
	})
}

func healthHandler(w http.ResponseWriter, _ *http.Request) error {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, err := w.Write([]byte(`{"status":"ok"}`))
	return err
}

func respondError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	body, err := json.Marshal(errorResponse{Error: msg})
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	_, _ = w.Write(body)
}

func main() {
	mux := http.NewServeMux()
	mux.Handle("/healthz", errorMiddleware(healthHandler))

	server := &http.Server{
		Addr:    ":8080",
		Handler: mux,
	}

	log.Println("starting server on :8080")
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("server error: %v", err)
	}
}
