package main

import (
	"context"
	"log"
	"net/http"
	"os/signal"
	"syscall"

	"avito-internship-task/internal/app"
	"avito-internship-task/internal/config"
)

func main() {
	cfg := config.Load()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	application, err := app.New(ctx, cfg)
	if err != nil {
		log.Fatalf("init error: %v", err)
	}

	errCh := make(chan error, 1)
	go func() {
		errCh <- application.Run()
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

	if err := application.Shutdown(shutdownCtx); err != nil && err != http.ErrServerClosed {
		log.Fatalf("shutdown error: %v", err)
	}
}
