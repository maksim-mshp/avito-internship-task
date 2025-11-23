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

	createBody := map[string]any{
		"team_name": "backend",
		"members": []map[string]any{
			{"user_id": "u1", "username": "Alice", "is_active": true},
			{"user_id": "u2", "username": "Bob", "is_active": false},
		},
	}
	runRequest(t, client, http.MethodPost, server.URL+"/team/add", createBody, http.StatusCreated)
	// duplicate team
	resp := runRequest(t, client, http.MethodPost, server.URL+"/team/add", createBody, http.StatusBadRequest)
	assertErrorCode(t, resp, "TEAM_EXISTS")

	// get ok
	resp = runRequest(t, client, http.MethodGet, server.URL+"/team/get?team_name=backend", nil, http.StatusOK)
	require.NotNil(t, resp)
	resp.Body.Close()

	// get not found
	resp = runRequest(t, client, http.MethodGet, server.URL+"/team/get?team_name=missing", nil, http.StatusNotFound)
	resp.Body.Close()

	// member exists in another team (unique user_id)
	another := map[string]any{
		"team_name": "another",
		"members": []map[string]any{
			{"user_id": "u1", "username": "Alice", "is_active": true},
		},
	}
	resp = runRequest(t, client, http.MethodPost, server.URL+"/team/add", another, http.StatusBadRequest)
	assertErrorCode(t, resp, "MEMBER_EXISTS")
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
	runRequest(t, client, http.MethodPost, server.URL+"/pullRequest/create", createReq, http.StatusCreated)
	resp := runRequest(t, client, http.MethodPost, server.URL+"/pullRequest/create", createReq, http.StatusConflict)
	assertErrorCode(t, resp, "PR_EXISTS")

	mergeReq := map[string]any{"pull_request_id": "pr1"}
	runRequest(t, client, http.MethodPost, server.URL+"/pullRequest/merge", mergeReq, http.StatusOK)
	runRequest(t, client, http.MethodPost, server.URL+"/pullRequest/merge", mergeReq, http.StatusOK)

	reassignReq := map[string]any{"pull_request_id": "pr1", "old_reviewer_id": "u2"}
	resp = runRequest(t, client, http.MethodPost, server.URL+"/pullRequest/reassign", reassignReq, http.StatusConflict)
	assertErrorCode(t, resp, "PR_MERGED")

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

func runRequest(t *testing.T, client *http.Client, method, url string, body any, status int) *http.Response {
	var rdr *bytes.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		rdr = bytes.NewReader(b)
	} else {
		rdr = bytes.NewReader(nil)
	}
	req, _ := http.NewRequest(method, url, rdr)
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	require.NoError(t, err)
	require.Equal(t, status, resp.StatusCode)
	require.Contains(t, resp.Header.Get("Content-Type"), "application/json")
	return resp
}

func assertErrorCode(t *testing.T, resp *http.Response, code string) {
	defer resp.Body.Close()
	var payload struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&payload)
	require.Equal(t, code, payload.Error.Code)
}
