package app

import (
	"context"
	"net/http"

	"avito-internship-task/internal/config"
	"avito-internship-task/internal/db"
	"avito-internship-task/internal/httpserver"
	"avito-internship-task/internal/pullrequests"
	"avito-internship-task/internal/teams"
	"avito-internship-task/internal/users"
)

type App struct {
	server *http.Server
	pool   closable
}

type closable interface {
	Close()
}

func New(ctx context.Context, cfg config.Config) (*App, error) {
	pool, err := db.Connect(ctx, cfg.DBURL)
	if err != nil {
		return nil, err
	}

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
	mux.Handle("/healthz", httpserver.WithError(func(w http.ResponseWriter, _ *http.Request) error {
		httpserver.RespondJSON(w, http.StatusOK, map[string]string{"status": "ok"})
		return nil
	}))
	teamHandler.Register(mux)
	userHandler.Register(mux)
	prHandler.Register(mux)

	server := &http.Server{
		Addr:    cfg.HTTPAddr,
		Handler: httpserver.Logging(mux),
	}

	return &App{
		server: server,
		pool:   pool,
	}, nil
}

func (a *App) Run() error {
	return a.server.ListenAndServe()
}

func (a *App) Shutdown(ctx context.Context) error {
	if a.pool != nil {
		a.pool.Close()
	}
	return a.server.Shutdown(ctx)
}
