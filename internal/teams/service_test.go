package teams

import (
	"context"
	"errors"
	"testing"

	"avito-internship-task/internal/entity"
	"github.com/jackc/pgconn"
	"github.com/stretchr/testify/require"
)

type teamRepoStub struct {
	teams           map[string]entity.Team
	existingMembers map[string]struct{}
}

func newTeamRepoStub() *teamRepoStub {
	return &teamRepoStub{
		teams:           make(map[string]entity.Team),
		existingMembers: make(map[string]struct{}),
	}
}

func (r *teamRepoStub) Create(ctx context.Context, team entity.Team) error {
	if _, ok := r.teams[team.TeamName]; ok {
		return &pgconn.PgError{Code: "23505", TableName: "teams"}
	}
	for _, m := range team.Members {
		if _, ok := r.existingMembers[m.UserID]; ok {
			return &pgconn.PgError{Code: "23505", TableName: "users"}
		}
	}
	r.teams[team.TeamName] = team
	for _, m := range team.Members {
		r.existingMembers[m.UserID] = struct{}{}
	}
	return nil
}

func (r *teamRepoStub) Get(ctx context.Context, name string) (entity.Team, error) {
	team, ok := r.teams[name]
	if !ok {
		return entity.Team{}, ErrNotFound
	}
	return team, nil
}

func TestServiceCreate(t *testing.T) {
	ctx := context.Background()
	tests := []struct {
		name      string
		repo      *teamRepoStub
		input     entity.Team
		wantErr   error
		wantCount int
	}{
		{
			name: "ok",
			repo: newTeamRepoStub(),
			input: entity.Team{
				TeamName: "backend",
				Members:  []entity.TeamMember{{UserID: "u1", Username: "Alice", IsActive: true}},
			},
			wantErr:   nil,
			wantCount: 1,
		},
		{
			name:      "invalid input",
			repo:      newTeamRepoStub(),
			input:     entity.Team{},
			wantErr:   ErrInvalidInput,
			wantCount: 0,
		},
		{
			name: "duplicate",
			repo: func() *teamRepoStub {
				r := newTeamRepoStub()
				r.teams["backend"] = entity.Team{TeamName: "backend"}
				return r
			}(),
			input:     entity.Team{TeamName: "backend", Members: []entity.TeamMember{{UserID: "u2", Username: "Bob"}}},
			wantErr:   ErrTeamExists,
			wantCount: 1,
		},
		{
			name: "member exists",
			repo: func() *teamRepoStub {
				r := newTeamRepoStub()
				r.teams["other"] = entity.Team{TeamName: "other"}
				r.existingMembers["u1"] = struct{}{}
				return r
			}(),
			input:     entity.Team{TeamName: "backend", Members: []entity.TeamMember{{UserID: "u1", Username: "Alice"}}},
			wantErr:   ErrMemberExists,
			wantCount: 1,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			svc := NewService(tt.repo)
			team, err := svc.Create(ctx, tt.input)
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.input.TeamName, team.TeamName)
			require.Equal(t, tt.wantCount, len(team.Members))
		})
	}
}

func TestServiceGet(t *testing.T) {
	ctx := context.Background()
	baseRepo := newTeamRepoStub()
	baseRepo.teams["backend"] = entity.Team{TeamName: "backend"}
	tests := []struct {
		name    string
		repo    *teamRepoStub
		input   string
		wantErr error
	}{
		{
			name:    "ok",
			repo:    baseRepo,
			input:   "backend",
			wantErr: nil,
		},
		{
			name:    "invalid",
			repo:    baseRepo,
			input:   "",
			wantErr: ErrInvalidInput,
		},
		{
			name:    "not found",
			repo:    baseRepo,
			input:   "missing",
			wantErr: ErrNotFound,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			svc := NewService(tt.repo)
			team, err := svc.Get(ctx, tt.input)
			if tt.wantErr != nil {
				require.True(t, errors.Is(err, tt.wantErr))
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.input, team.TeamName)
		})
	}
}
