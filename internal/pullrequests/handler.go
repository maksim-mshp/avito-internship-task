package pullrequests

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"avito-internship-task/internal/entity"
	"avito-internship-task/internal/httpserver"
	"github.com/jackc/pgconn"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) Register(mux *http.ServeMux) {
	mux.Handle("/pullRequest/create", httpserver.WithError(h.create))
	mux.Handle("/pullRequest/merge", httpserver.WithError(h.merge))
	mux.Handle("/pullRequest/reassign", httpserver.WithError(h.reassign))
}

type createRequest struct {
	PullRequestID   string `json:"pull_request_id"`
	PullRequestName string `json:"pull_request_name"`
	AuthorID        string `json:"author_id"`
}

type mergeRequest struct {
	PullRequestID string `json:"pull_request_id"`
}

type reassignRequest struct {
	PullRequestID string `json:"pull_request_id"`
	OldUserID     string `json:"old_user_id"`
}

type prEnvelope struct {
	PR entity.PullRequest `json:"pr"`
}

type reassignResponse struct {
	PR         entity.PullRequest `json:"pr"`
	ReplacedBy string             `json:"replaced_by"`
}

type errorEnvelope struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

const (
	codeBadRequest  = "BAD_REQUEST"
	codeNotFound    = "NOT_FOUND"
	codePRExists    = "PR_EXISTS"
	codePRMerged    = "PR_MERGED"
	codeNotAssigned = "NOT_ASSIGNED"
	codeNoCandidate = "NO_CANDIDATE"
)

func (h *Handler) create(w http.ResponseWriter, r *http.Request) error {
	if r.Method != http.MethodPost {
		httpserver.RespondError(w, http.StatusMethodNotAllowed, "method not allowed")
		return nil
	}
	var req createRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writePRError(w, http.StatusBadRequest, codeBadRequest, "invalid json")
		return nil
	}
	pr, err := h.service.Create(r.Context(), entity.PullRequest{
		PullRequestID:   req.PullRequestID,
		PullRequestName: req.PullRequestName,
		AuthorID:        req.AuthorID,
	})
	if err != nil {
		if isPGUnique(err) || isDuplicateErr(err) {
			writePRError(w, http.StatusConflict, codePRExists, "PR id already exists")
			return nil
		}
		switch {
		case errors.Is(err, ErrInvalidInput):
			writePRError(w, http.StatusBadRequest, codeBadRequest, "pull_request_id, pull_request_name and author_id are required")
			return nil
		case errors.Is(err, ErrNotFound):
			writePRError(w, http.StatusNotFound, codeNotFound, "author not found or inactive")
			return nil
		case errors.Is(err, ErrExists):
			writePRError(w, http.StatusConflict, codePRExists, "PR id already exists")
			return nil
		default:
			return err
		}
	}
	httpserver.RespondJSON(w, http.StatusCreated, prEnvelope{PR: pr})
	return nil
}

func (h *Handler) merge(w http.ResponseWriter, r *http.Request) error {
	if r.Method != http.MethodPost {
		httpserver.RespondError(w, http.StatusMethodNotAllowed, "method not allowed")
		return nil
	}
	var req mergeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writePRError(w, http.StatusBadRequest, codeBadRequest, "invalid json")
		return nil
	}
	pr, err := h.service.Merge(r.Context(), req.PullRequestID)
	if err != nil {
		switch {
		case errors.Is(err, ErrInvalidInput):
			writePRError(w, http.StatusBadRequest, codeBadRequest, "pull_request_id is required")
			return nil
		case errors.Is(err, ErrNotFound):
			writePRError(w, http.StatusNotFound, codeNotFound, "PR not found")
			return nil
		default:
			return err
		}
	}
	httpserver.RespondJSON(w, http.StatusOK, prEnvelope{PR: pr})
	return nil
}

func (h *Handler) reassign(w http.ResponseWriter, r *http.Request) error {
	if r.Method != http.MethodPost {
		httpserver.RespondError(w, http.StatusMethodNotAllowed, "method not allowed")
		return nil
	}
	var req reassignRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writePRError(w, http.StatusBadRequest, codeBadRequest, "invalid json")
		return nil
	}
	pr, replacement, err := h.service.Reassign(r.Context(), req.PullRequestID, req.OldUserID)
	if err != nil {
		if isPGUnique(err) || isDuplicateErr(err) {
			writePRError(w, http.StatusConflict, codePRExists, "PR id already exists")
			return nil
		}
		switch {
		case errors.Is(err, ErrInvalidInput):
			writePRError(w, http.StatusBadRequest, codeBadRequest, "pull_request_id and old_user_id are required")
			return nil
		case errors.Is(err, ErrNotFound):
			writePRError(w, http.StatusNotFound, codeNotFound, "resource not found")
			return nil
		case errors.Is(err, ErrMerged):
			writePRError(w, http.StatusConflict, codePRMerged, "cannot reassign on merged PR")
			return nil
		case errors.Is(err, ErrNotAssigned):
			writePRError(w, http.StatusConflict, codeNotAssigned, "reviewer is not assigned to this PR")
			return nil
		case errors.Is(err, ErrNoCandidate):
			writePRError(w, http.StatusConflict, codeNoCandidate, "no active replacement candidate in team")
			return nil
		default:
			return err
		}
	}
	httpserver.RespondJSON(w, http.StatusOK, reassignResponse{PR: pr, ReplacedBy: replacement})
	return nil
}

func writePRError(w http.ResponseWriter, status int, code, message string) {
	var e errorEnvelope
	e.Error.Code = code
	e.Error.Message = message
	httpserver.RespondJSON(w, status, e)
}

func isPGUnique(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == "23505"
	}
	return false
}

func isDuplicateErr(err error) bool {
	return strings.Contains(err.Error(), "duplicate key value")
}
