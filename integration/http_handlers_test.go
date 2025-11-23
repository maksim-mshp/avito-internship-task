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

	tests := []struct {
		name   string
		method string
		url    string
		body   any
		status int
	}{
		{
			name:   "create ok",
			method: http.MethodPost,
			url:    "/team/add",
			body: map[string]any{
				"team_name": "backend",
				"members": []map[string]any{
					{"user_id": "u1", "username": "Alice", "is_active": true},
					{"user_id": "u2", "username": "Bob", "is_active": false},
				},
			},
			status: http.StatusCreated,
		},
		{
			name:   "get ok",
			method: http.MethodGet,
			url:    "/team/get?team_name=backend",
			status: http.StatusOK,
		},
		{
			name:   "get not found",
			method: http.MethodGet,
			url:    "/team/get?team_name=missing",
			status: http.StatusNotFound,
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			var reqBody *bytes.Reader
			if tt.body != nil {
				b, _ := json.Marshal(tt.body)
				reqBody = bytes.NewReader(b)
			} else {
				reqBody = bytes.NewReader(nil)
			}
			req, _ := http.NewRequest(tt.method, server.URL+tt.url, reqBody)
			req.Header.Set("Content-Type", "application/json")
			resp, err := client.Do(req)
			require.NoError(t, err)
			defer resp.Body.Close()
			require.Equal(t, tt.status, resp.StatusCode)
			require.Contains(t, resp.Header.Get("Content-Type"), "application/json")
		})
	}
}

func TestUserHandlers(t *testing.T) {
	server, cleanup := startTestServer(t)
	defer cleanup()
	client := &http.Client{Timeout: 5 * time.Second}

	createTeam(client, server.URL)

	tests := []struct {
		name   string
		body   any
		status int
	}{
		{
			name:   "ok",
			body:   map[string]any{"user_id": "u1", "is_active": false},
			status: http.StatusOK,
		},
		{
			name:   "not found",
			body:   map[string]any{"user_id": "missing", "is_active": true},
			status: http.StatusNotFound,
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			b, _ := json.Marshal(tt.body)
			resp, err := client.Post(server.URL+"/users/setIsActive", "application/json", bytes.NewReader(b))
			require.NoError(t, err)
			defer resp.Body.Close()
			require.Equal(t, tt.status, resp.StatusCode)
			require.Contains(t, resp.Header.Get("Content-Type"), "application/json")
		})
	}
}

func TestPullRequestHandlers(t *testing.T) {
	server, cleanup := startTestServer(t)
	defer cleanup()
	client := &http.Client{Timeout: 5 * time.Second}

	createTeam(client, server.URL)

	createReq := map[string]any{
		"pull_request_id":   "pr1",
		"pull_request_name": "Add feature",
		"author_id":         "u1",
	}
	runRequest(t, client, server.URL+"/pullRequest/create", createReq, http.StatusCreated)
	runRequest(t, client, server.URL+"/pullRequest/create", createReq, http.StatusConflict)

	mergeReq := map[string]any{"pull_request_id": "pr1"}
	runRequest(t, client, server.URL+"/pullRequest/merge", mergeReq, http.StatusOK)
	runRequest(t, client, server.URL+"/pullRequest/merge", mergeReq, http.StatusOK)

	reassignReq := map[string]any{"pull_request_id": "pr1", "old_user_id": "u2"}
	runRequest(t, client, server.URL+"/pullRequest/reassign", reassignReq, http.StatusConflict)

	resp, err := client.Get(server.URL + "/pullRequest/stats")
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)
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

func runRequest(t *testing.T, client *http.Client, url string, body any, status int) {
	b, _ := json.Marshal(body)
	resp, err := client.Post(url, "application/json", bytes.NewReader(b))
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, status, resp.StatusCode)
	require.Contains(t, resp.Header.Get("Content-Type"), "application/json")
}
