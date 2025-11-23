package pullrequests

import (
	"context"
	"errors"
	"math/rand"
	"strings"
	"time"

	"avito-internship-task/internal/entity"
	"github.com/jackc/pgconn"
)

var (
	ErrInvalidInput = errors.New("invalid input")
	ErrNotFound     = errors.New("not found")
	ErrExists       = errors.New("pr exists")
	ErrMerged       = errors.New("pr merged")
	ErrNotAssigned  = errors.New("not assigned")
	ErrNoCandidate  = errors.New("no candidate")
)

type Service struct {
	repo Repo
	rand *rand.Rand
}

type Repo interface {
	GetUser(ctx context.Context, userID string) (entity.User, error)
	GetActiveTeamMembers(ctx context.Context, teamName string) ([]entity.User, error)
	Create(ctx context.Context, pr entity.PullRequest) error
	Get(ctx context.Context, id string) (entity.PullRequest, error)
	Merge(ctx context.Context, id string, ts time.Time) error
	ReplaceReviewer(ctx context.Context, prID, oldID, newID string) error
}

func NewService(repo Repo) *Service {
	return &Service{
		repo: repo,
		rand: rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

func (s *Service) Create(ctx context.Context, pr entity.PullRequest) (entity.PullRequest, error) {
	pr.PullRequestID = strings.TrimSpace(pr.PullRequestID)
	pr.PullRequestName = strings.TrimSpace(pr.PullRequestName)
	pr.AuthorID = strings.TrimSpace(pr.AuthorID)
	if pr.PullRequestID == "" || pr.PullRequestName == "" || pr.AuthorID == "" {
		return entity.PullRequest{}, ErrInvalidInput
	}

	author, err := s.repo.GetUser(ctx, pr.AuthorID)
	if err != nil {
		if isNotFound(err) {
			return entity.PullRequest{}, ErrNotFound
		}
		return entity.PullRequest{}, err
	}
	if !author.IsActive {
		return entity.PullRequest{}, ErrNotFound
	}

	candidates, err := s.repo.GetActiveTeamMembers(ctx, author.TeamName)
	if err != nil {
		return entity.PullRequest{}, err
	}
	filtered := make([]entity.User, 0, len(candidates))
	for _, u := range candidates {
		if u.UserID == pr.AuthorID {
			continue
		}
		filtered = append(filtered, u)
	}
	selected := s.pickRandom(filtered, 2)
	pr.Status = "OPEN"
	pr.Assigned = selected

	if err := s.repo.Create(ctx, pr); err != nil {
		if isUniqueViolation(err) {
			return entity.PullRequest{}, ErrExists
		}
		return entity.PullRequest{}, err
	}
	return pr, nil
}

func (s *Service) Merge(ctx context.Context, id string) (entity.PullRequest, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return entity.PullRequest{}, ErrInvalidInput
	}
	pr, err := s.repo.Get(ctx, id)
	if err != nil {
		if isNotFound(err) {
			return entity.PullRequest{}, ErrNotFound
		}
		return entity.PullRequest{}, err
	}
	if pr.Status == "MERGED" {
		return pr, nil
	}
	now := time.Now().UTC()
	if err := s.repo.Merge(ctx, id, now); err != nil {
		return entity.PullRequest{}, err
	}
	pr.Status = "MERGED"
	pr.MergedAt = &now
	return pr, nil
}

func (s *Service) Reassign(ctx context.Context, prID, oldReviewer string) (entity.PullRequest, string, error) {
	prID = strings.TrimSpace(prID)
	oldReviewer = strings.TrimSpace(oldReviewer)
	if prID == "" || oldReviewer == "" {
		return entity.PullRequest{}, "", ErrInvalidInput
	}
	pr, err := s.repo.Get(ctx, prID)
	if err != nil {
		if isNotFound(err) {
			return entity.PullRequest{}, "", ErrNotFound
		}
		return entity.PullRequest{}, "", err
	}
	if pr.Status == "MERGED" {
		return entity.PullRequest{}, "", ErrMerged
	}
	found := false
	for _, r := range pr.Assigned {
		if r == oldReviewer {
			found = true
			break
		}
	}
	if !found {
		return entity.PullRequest{}, "", ErrNotAssigned
	}
	reviewer, err := s.repo.GetUser(ctx, oldReviewer)
	if err != nil {
		if isNotFound(err) {
			return entity.PullRequest{}, "", ErrNotFound
		}
		return entity.PullRequest{}, "", err
	}
	if !reviewer.IsActive {
		return entity.PullRequest{}, "", ErrNotFound
	}
	candidates, err := s.repo.GetActiveTeamMembers(ctx, reviewer.TeamName)
	if err != nil {
		return entity.PullRequest{}, "", err
	}
	filtered := make([]entity.User, 0, len(candidates))
	assignedSet := make(map[string]struct{}, len(pr.Assigned))
	for _, r := range pr.Assigned {
		assignedSet[r] = struct{}{}
	}
	assignedSet[pr.AuthorID] = struct{}{}

	for _, c := range candidates {
		if c.UserID == oldReviewer {
			continue
		}
		if _, exists := assignedSet[c.UserID]; exists {
			continue
		}
		filtered = append(filtered, c)
	}
	if len(filtered) == 0 {
		return entity.PullRequest{}, "", ErrNoCandidate
	}
	replacement := s.pickRandom(filtered, 1)
	if err := s.repo.ReplaceReviewer(ctx, prID, oldReviewer, replacement[0]); err != nil {
		return entity.PullRequest{}, "", err
	}
	pr, err = s.repo.Get(ctx, prID)
	if err != nil {
		return entity.PullRequest{}, "", err
	}
	return pr, replacement[0], nil
}

func (s *Service) pickRandom(users []entity.User, count int) []string {
	if len(users) == 0 || count == 0 {
		return nil
	}
	if len(users) <= count {
		ids := make([]string, 0, len(users))
		for _, u := range users {
			ids = append(ids, u.UserID)
		}
		return ids
	}
	perm := s.rand.Perm(len(users))
	selected := make([]string, 0, count)
	for i := 0; i < count; i++ {
		selected = append(selected, users[perm[i]].UserID)
	}
	return selected
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == "23505"
	}
	return false
}
