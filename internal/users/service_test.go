package users

import (
	"context"
	"errors"
	"testing"

	"avito-internship-task/internal/entity"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"
)

type userRepoStub struct {
	users  map[string]entity.User
	review map[string][]entity.PullRequestShort
}

func newUserRepoStub() *userRepoStub {
	return &userRepoStub{
		users:  make(map[string]entity.User),
		review: make(map[string][]entity.PullRequestShort),
	}
}

func (r *userRepoStub) SetIsActive(ctx context.Context, userID string, active bool) (entity.User, error) {
	u, ok := r.users[userID]
	if !ok {
		return entity.User{}, pgx.ErrNoRows
	}
	u.IsActive = active
	r.users[userID] = u
	return u, nil
}

func (r *userRepoStub) Get(ctx context.Context, userID string) (entity.User, error) {
	u, ok := r.users[userID]
	if !ok {
		return entity.User{}, pgx.ErrNoRows
	}
	return u, nil
}

func (r *userRepoStub) GetReview(ctx context.Context, userID string) ([]entity.PullRequestShort, error) {
	return r.review[userID], nil
}

func TestSetIsActive(t *testing.T) {
	ctx := context.Background()
	repo := newUserRepoStub()
	repo.users["u1"] = entity.User{UserID: "u1", Username: "Alice", TeamName: "backend", IsActive: true}
	svc := NewService(repo)

	tests := []struct {
		name    string
		userID  string
		active  bool
		wantErr error
	}{
		{name: "ok", userID: "u1", active: false},
		{name: "invalid", userID: "", active: true, wantErr: ErrInvalidInput},
		{name: "not found", userID: "missing", active: true, wantErr: ErrNotFound},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			user, err := svc.SetIsActive(ctx, tt.userID, tt.active)
			if tt.wantErr != nil {
				require.True(t, errors.Is(err, tt.wantErr))
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.active, user.IsActive)
		})
	}
}

func TestGetReview(t *testing.T) {
	ctx := context.Background()
	repo := newUserRepoStub()
	repo.users["u1"] = entity.User{UserID: "u1"}
	repo.review["u1"] = []entity.PullRequestShort{{PullRequestID: "pr1"}}
	svc := NewService(repo)

	tests := []struct {
		name    string
		userID  string
		wantLen int
		wantErr error
	}{
		{name: "ok", userID: "u1", wantLen: 1},
		{name: "invalid", userID: "", wantErr: ErrInvalidInput},
		{name: "not found", userID: "missing", wantErr: ErrNotFound},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			items, err := svc.GetReview(ctx, tt.userID)
			if tt.wantErr != nil {
				require.True(t, errors.Is(err, tt.wantErr))
				return
			}
			require.NoError(t, err)
			require.Len(t, items, tt.wantLen)
		})
	}
}
