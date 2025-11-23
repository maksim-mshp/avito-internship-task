package integration

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"avito-internship-task/internal/httpserver"
	"avito-internship-task/internal/pullrequests"
	"avito-internship-task/internal/teams"
	"avito-internship-task/internal/users"
	"github.com/stretchr/testify/require"
)

func startTestServer(t *testing.T) (*httptest.Server, func()) {
	pool := setupPostgres(t)

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

	server := httptest.NewServer(httpserver.Logging(mux))
	cleanup := func() {
		server.Close()
		pool.Close()
	}
	return server, cleanup
}

func TestTeamHandlers(t *testing.T) {
	server, cleanup := startTestServer(t)
	defer cleanup()
	client := &http.Client{Timeout: 5 * time.Second}

	body := map[string]any{
		"team_name": "backend",
		"members": []map[string]any{
			{"user_id": "u1", "username": "Alice", "is_active": true},
			{"user_id": "u2", "username": "Bob", "is_active": false},
		},
	}
	b, _ := json.Marshal(body)
	resp, err := client.Post(server.URL+"/team/add", "application/json", bytes.NewReader(b))
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusCreated, resp.StatusCode)

	resp, err = client.Get(server.URL + "/team/get?team_name=backend")
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	resp, err = client.Get(server.URL + "/team/get?team_name=missing")
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusNotFound, resp.StatusCode)
}

func TestUserHandlers(t *testing.T) {
	server, cleanup := startTestServer(t)
	defer cleanup()
	client := &http.Client{Timeout: 5 * time.Second}

	createTeam(client, server.URL)

	reqBody := map[string]any{"user_id": "u1", "is_active": false}
	b, _ := json.Marshal(reqBody)
	resp, err := client.Post(server.URL+"/users/setIsActive", "application/json", bytes.NewReader(b))
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	reqBody = map[string]any{"user_id": "missing", "is_active": true}
	b, _ = json.Marshal(reqBody)
	resp, err = client.Post(server.URL+"/users/setIsActive", "application/json", bytes.NewReader(b))
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusNotFound, resp.StatusCode)
}

func TestPullRequestHandlers(t *testing.T) {
	server, cleanup := startTestServer(t)
	defer cleanup()
	client := &http.Client{Timeout: 5 * time.Second}

	createTeam(client, server.URL)

	prBody := map[string]any{
		"pull_request_id":   "pr1",
		"pull_request_name": "Add feature",
		"author_id":         "u1",
	}
	b, _ := json.Marshal(prBody)
	resp, err := client.Post(server.URL+"/pullRequest/create", "application/json", bytes.NewReader(b))
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusCreated, resp.StatusCode)

	mergeBody := map[string]any{"pull_request_id": "pr1"}
	b, _ = json.Marshal(mergeBody)
	resp, err = client.Post(server.URL+"/pullRequest/merge", "application/json", bytes.NewReader(b))
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	resp, err = client.Post(server.URL+"/pullRequest/merge", "application/json", bytes.NewReader(b))
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	reassignBody := map[string]any{"pull_request_id": "pr1", "old_user_id": "u2"}
	b, _ = json.Marshal(reassignBody)
	resp, err = client.Post(server.URL+"/pullRequest/reassign", "application/json", bytes.NewReader(b))
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusConflict, resp.StatusCode)
}

func createTeam(client *http.Client, baseURL string) {
	body := map[string]any{
		"team_name": "backend",
		"members": []map[string]any{
			{"user_id": "u1", "username": "Alice", "is_active": true},
			{"user_id": "u2", "username": "Bob", "is_active": true},
		},
	}
	b, _ := json.Marshal(body)
	resp, _ := client.Post(baseURL+"/team/add", "application/json", bytes.NewReader(b))
	if resp != nil {
		resp.Body.Close()
	}
}
