package users

import (
	"context"
	"errors"
	"strings"

	"avito-internship-task/internal/entity"
)

var (
	ErrInvalidInput = errors.New("invalid input")
	ErrNotFound     = errors.New("not found")
)

type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) SetIsActive(ctx context.Context, userID string, active bool) (entity.User, error) {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return entity.User{}, ErrInvalidInput
	}
	user, err := s.repo.SetIsActive(ctx, userID, active)
	if err != nil {
		if isNotFound(err) {
			return entity.User{}, ErrNotFound
		}
		return entity.User{}, err
	}
	return user, nil
}

func (s *Service) GetReview(ctx context.Context, userID string) ([]entity.PullRequestShort, error) {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return nil, ErrInvalidInput
	}
	if _, err := s.repo.Get(ctx, userID); err != nil {
		if isNotFound(err) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return s.repo.GetReview(ctx, userID)
}
