package teams

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
	mux.Handle("/team/add", httpserver.WithError(h.createTeam))
	mux.Handle("/team/get", httpserver.WithError(h.getTeam))
}

type createTeamRequest struct {
	TeamName string              `json:"team_name"`
	Members  []entity.TeamMember `json:"members"`
}

type teamEnvelope struct {
	Team TeamResponse `json:"team"`
}

type errorCode string

const (
	errorTeamExists   errorCode = "TEAM_EXISTS"
	errorNotFound     errorCode = "NOT_FOUND"
	errorBadRequest   errorCode = "BAD_REQUEST"
	errorMemberExists errorCode = "MEMBER_EXISTS"
)

type errorResponse struct {
	Error struct {
		Code    errorCode `json:"code"`
		Message string    `json:"message"`
	} `json:"error"`
}

func (h *Handler) createTeam(w http.ResponseWriter, r *http.Request) error {
	if r.Method != http.MethodPost {
		httpserver.RespondError(w, http.StatusMethodNotAllowed, "method not allowed")
		return nil
	}
	var req createTeamRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, errorBadRequest, "invalid json")
		return nil
	}
	team := entity.Team{
		TeamName: req.TeamName,
		Members:  req.Members,
	}
	team, err := h.service.Create(r.Context(), team)
	if err != nil {
		switch {
		case isMemberDuplicate(err):
			writeError(w, http.StatusBadRequest, errorMemberExists, "user_id already exists")
			return nil
		case errors.Is(err, ErrTeamExists):
			writeError(w, http.StatusBadRequest, errorTeamExists, "team_name already exists")
			return nil
		case errors.Is(err, ErrMemberExists):
			writeError(w, http.StatusBadRequest, errorMemberExists, "user_id already exists")
			return nil
		case errors.Is(err, ErrInvalidInput):
			writeError(w, http.StatusBadRequest, errorBadRequest, "team_name or members are invalid")
			return nil
		default:
			if strings.Contains(err.Error(), "duplicate key value") {
				writeError(w, http.StatusBadRequest, errorTeamExists, "team_name already exists")
				return nil
			}
			return err
		}
	}
	httpserver.RespondJSON(w, http.StatusCreated, teamEnvelope{Team: toResponse(team)})
	return nil
}

func (h *Handler) getTeam(w http.ResponseWriter, r *http.Request) error {
	if r.Method != http.MethodGet {
		httpserver.RespondError(w, http.StatusMethodNotAllowed, "method not allowed")
		return nil
	}
	teamName := r.URL.Query().Get("team_name")
	team, err := h.service.Get(r.Context(), teamName)
	if err != nil {
		switch {
		case errors.Is(err, ErrInvalidInput), errors.Is(err, ErrNotFound):
			writeError(w, http.StatusNotFound, errorNotFound, "team not found")
			return nil
		default:
			return err
		}
	}
	httpserver.RespondJSON(w, http.StatusOK, toResponse(team))
	return nil
}

type TeamResponse struct {
	TeamName string              `json:"team_name"`
	Members  []entity.TeamMember `json:"members"`
}

func toResponse(t entity.Team) TeamResponse {
	return TeamResponse{
		TeamName: t.TeamName,
		Members:  t.Members,
	}
}

func writeError(w http.ResponseWriter, status int, code errorCode, message string) {
	var resp errorResponse
	resp.Error.Code = code
	resp.Error.Message = message
	httpserver.RespondJSON(w, status, resp)
}

func isUniqueErr(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return true
	}
	return strings.Contains(err.Error(), "duplicate key value")
}

func isMemberDuplicate(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return strings.Contains(pgErr.ConstraintName, "users") || strings.Contains(pgErr.Message, "users") || strings.Contains(pgErr.Message, "user_id")
	}
	return strings.Contains(err.Error(), "users") || strings.Contains(err.Error(), "user_id")
}
