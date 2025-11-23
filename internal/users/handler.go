package users

import (
	"encoding/json"
	"net/http"

	"avito-internship-task/internal/entity"
	"avito-internship-task/internal/httpserver"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) Register(mux *http.ServeMux) {
	mux.Handle("/users/setIsActive", httpserver.WithError(h.setIsActive))
	mux.Handle("/users/getReview", httpserver.WithError(h.getReview))
}

type setActiveRequest struct {
	UserID   string `json:"user_id"`
	IsActive bool   `json:"is_active"`
}

type userEnvelope struct {
	User entity.User `json:"user"`
}

type reviewResponse struct {
	UserID       string                    `json:"user_id"`
	PullRequests []entity.PullRequestShort `json:"pull_requests"`
}

func (h *Handler) setIsActive(w http.ResponseWriter, r *http.Request) error {
	if r.Method != http.MethodPost {
		httpserver.RespondError(w, http.StatusMethodNotAllowed, "method not allowed")
		return nil
	}
	var req setActiveRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeUserError(w, http.StatusBadRequest, "NOT_FOUND", "invalid json")
		return nil
	}
	user, err := h.service.SetIsActive(r.Context(), req.UserID, req.IsActive)
	if err != nil {
		switch err {
		case ErrInvalidInput:
			writeUserError(w, http.StatusBadRequest, "NOT_FOUND", "user_id is required")
			return nil
		case ErrNotFound:
			writeUserError(w, http.StatusNotFound, "NOT_FOUND", "user not found")
			return nil
		default:
			return err
		}
	}
	httpserver.RespondJSON(w, http.StatusOK, userEnvelope{User: user})
	return nil
}

func (h *Handler) getReview(w http.ResponseWriter, r *http.Request) error {
	if r.Method != http.MethodGet {
		httpserver.RespondError(w, http.StatusMethodNotAllowed, "method not allowed")
		return nil
	}
	userID := r.URL.Query().Get("user_id")
	prs, err := h.service.GetReview(r.Context(), userID)
	if err != nil {
		switch err {
		case ErrInvalidInput:
			writeUserError(w, http.StatusBadRequest, "NOT_FOUND", "user_id is required")
			return nil
		case ErrNotFound:
			writeUserError(w, http.StatusNotFound, "NOT_FOUND", "user not found")
			return nil
		default:
			return err
		}
	}
	resp := reviewResponse{
		UserID:       userID,
		PullRequests: prs,
	}
	httpserver.RespondJSON(w, http.StatusOK, resp)
	return nil
}

type errorEnvelope struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

func writeUserError(w http.ResponseWriter, status int, code, message string) {
	var e errorEnvelope
	e.Error.Code = code
	e.Error.Message = message
	httpserver.RespondJSON(w, status, e)
}
