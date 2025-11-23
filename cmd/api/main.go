package main

import (
	"context"
	"log"
	"net/http"
	"os/signal"
	"syscall"

	"avito-internship-task/internal/config"
	"avito-internship-task/internal/db"
	"avito-internship-task/internal/httpserver"
	"avito-internship-task/internal/pullrequests"
	"avito-internship-task/internal/teams"
	"avito-internship-task/internal/users"
)

func main() {
	cfg := config.Load()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	pool, err := db.Connect(ctx, cfg.DBURL)
	if err != nil {
		log.Fatalf("db connect error: %v", err)
	}
	defer pool.Close()

	teamRepo := teams.NewRepository(pool)
	teamService := teams.NewService(teamRepo)
	teamHandler := teams.NewHandler(teamService)
	userRepo := users.NewRepository(pool)
	userService := users.NewService(userRepo)
	userHandler := users.NewHandler(userService)
	prRepo := pullrequests.NewRepository(pool)
	prService := pullrequests.NewService(prRepo)
	prHandler := pullrequests.NewHandler(prService)

	mux := http.NewServeMux()
	mux.Handle("/healthz", httpserver.WithError(healthHandler))
	teamHandler.Register(mux)
	userHandler.Register(mux)
	prHandler.Register(mux)

	server := &http.Server{
		Addr:    cfg.HTTPAddr,
		Handler: httpserver.Logging(mux),
	}

	errCh := make(chan error, 1)
	go func() {
		errCh <- server.ListenAndServe()
	}()

	select {
	case err := <-errCh:
		if err != nil && err != http.ErrServerClosed {
			log.Fatalf("server error: %v", err)
		}
	case <-ctx.Done():
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil && err != http.ErrServerClosed {
		log.Fatalf("shutdown error: %v", err)
	}
}

func healthHandler(w http.ResponseWriter, _ *http.Request) error {
	httpserver.RespondJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	return nil
}
