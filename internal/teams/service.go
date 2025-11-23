package teams

import (
	"context"
	"errors"
	"strings"

	"avito-internship-task/internal/entity"
	"github.com/jackc/pgconn"
)

var (
	ErrInvalidInput = errors.New("invalid input")
	ErrTeamExists   = errors.New("team exists")
	ErrNotFound     = errors.New("not found")
)

type Service struct {
	repo Repo
}

type Repo interface {
	Create(ctx context.Context, team entity.Team) error
	Get(ctx context.Context, name string) (entity.Team, error)
}

func NewService(repo Repo) *Service {
	return &Service{repo: repo}
}

func (s *Service) Create(ctx context.Context, team entity.Team) (entity.Team, error) {
	team.TeamName = strings.TrimSpace(team.TeamName)
	if team.TeamName == "" {
		return entity.Team{}, ErrInvalidInput
	}
	normalized := make([]entity.TeamMember, 0, len(team.Members))
	seen := make(map[string]struct{})
	for _, m := range team.Members {
		id := strings.TrimSpace(m.UserID)
		name := strings.TrimSpace(m.Username)
		if id == "" || name == "" {
			return entity.Team{}, ErrInvalidInput
		}
		if _, ok := seen[id]; ok {
			return entity.Team{}, ErrInvalidInput
		}
		seen[id] = struct{}{}
		normalized = append(normalized, entity.TeamMember{
			UserID:   id,
			Username: name,
			IsActive: m.IsActive,
		})
	}
	team.Members = normalized
	if err := s.repo.Create(ctx, team); err != nil {
		if isUniqueViolation(err) {
			return entity.Team{}, ErrTeamExists
		}
		return entity.Team{}, err
	}
	return team, nil
}

func (s *Service) Get(ctx context.Context, name string) (entity.Team, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return entity.Team{}, ErrInvalidInput
	}
	team, err := s.repo.Get(ctx, name)
	if err != nil {
		if isNotFound(err) {
			return entity.Team{}, ErrNotFound
		}
		return entity.Team{}, err
	}
	return team, nil
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == "23505"
	}
	return false
}
