package pullrequests

import (
	"context"
	"math/rand"
	"testing"
	"time"

	"avito-internship-task/internal/entity"
	"github.com/jackc/pgconn"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"
)

type prRepoStub struct {
	users     map[string]entity.User
	prs       map[string]entity.PullRequest
	reviewers map[string][]string
}

func newPRRepoStub() *prRepoStub {
	return &prRepoStub{
		users:     make(map[string]entity.User),
		prs:       make(map[string]entity.PullRequest),
		reviewers: make(map[string][]string),
	}
}

func (r *prRepoStub) GetUser(ctx context.Context, userID string) (entity.User, error) {
	u, ok := r.users[userID]
	if !ok {
		return entity.User{}, pgx.ErrNoRows
	}
	return u, nil
}

func (r *prRepoStub) GetActiveTeamMembers(ctx context.Context, teamName string) ([]entity.User, error) {
	result := make([]entity.User, 0)
	for _, u := range r.users {
		if u.TeamName == teamName && u.IsActive {
			result = append(result, u)
		}
	}
	return result, nil
}

func (r *prRepoStub) Create(ctx context.Context, pr entity.PullRequest) error {
	if _, ok := r.prs[pr.PullRequestID]; ok {
		return &pgconn.PgError{Code: "23505"}
	}
	r.prs[pr.PullRequestID] = pr
	r.reviewers[pr.PullRequestID] = append([]string{}, pr.Assigned...)
	return nil
}

func (r *prRepoStub) Get(ctx context.Context, id string) (entity.PullRequest, error) {
	pr, ok := r.prs[id]
	if !ok {
		return entity.PullRequest{}, pgx.ErrNoRows
	}
	revs := r.reviewers[id]
	pr.Assigned = append([]string{}, revs...)
	return pr, nil
}

func (r *prRepoStub) Merge(ctx context.Context, id string, ts time.Time) error {
	pr, ok := r.prs[id]
	if !ok {
		return pgx.ErrNoRows
	}
	pr.Status = "MERGED"
	if pr.MergedAt == nil {
		pr.MergedAt = &ts
	}
	r.prs[id] = pr
	return nil
}

func (r *prRepoStub) ReplaceReviewer(ctx context.Context, prID, oldID, newID string) error {
	revs := r.reviewers[prID]
	for i, v := range revs {
		if v == oldID {
			revs[i] = newID
			r.reviewers[prID] = revs
			return nil
		}
	}
	return nil
}

func (r *prRepoStub) StatsAssignments(ctx context.Context) (map[string]int, error) {
	result := make(map[string]int)
	for prID, revs := range r.reviewers {
		_ = prID
		for _, id := range revs {
			result[id]++
		}
	}
	return result, nil
}

func TestCreate(t *testing.T) {
	ctx := context.Background()
	repo := newPRRepoStub()
	repo.users["author"] = entity.User{UserID: "author", TeamName: "team", IsActive: true}
	repo.users["u1"] = entity.User{UserID: "u1", TeamName: "team", IsActive: true}
	repo.users["u2"] = entity.User{UserID: "u2", TeamName: "team", IsActive: true}
	repo.users["u3"] = entity.User{UserID: "u3", TeamName: "team", IsActive: false}

	tests := []struct {
		name    string
		input   entity.PullRequest
		wantErr error
	}{
		{
			name: "ok",
			input: entity.PullRequest{
				PullRequestID:   "pr1",
				PullRequestName: "Test",
				AuthorID:        "author",
			},
		},
		{
			name:    "invalid",
			input:   entity.PullRequest{},
			wantErr: ErrInvalidInput,
		},
		{
			name: "duplicate",
			input: entity.PullRequest{
				PullRequestID:   "pr1",
				PullRequestName: "Test",
				AuthorID:        "author",
			},
			wantErr: ErrExists,
		},
		{
			name: "author missing",
			input: entity.PullRequest{
				PullRequestID:   "pr2",
				PullRequestName: "X",
				AuthorID:        "missing",
			},
			wantErr: ErrNotFound,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			svc := NewService(repo)
			svc.rand = randSource(1)
			pr, err := svc.Create(ctx, tt.input)
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			require.Equal(t, "OPEN", pr.Status)
			require.LessOrEqual(t, len(pr.Assigned), 2)
			for _, r := range pr.Assigned {
				require.NotEqual(t, tt.input.AuthorID, r)
			}
		})
	}
}

func TestMerge(t *testing.T) {
	ctx := context.Background()
	repo := newPRRepoStub()
	repo.prs["pr1"] = entity.PullRequest{PullRequestID: "pr1", Status: "OPEN"}
	svc := NewService(repo)

	tests := []struct {
		name    string
		id      string
		wantErr error
	}{
		{name: "ok", id: "pr1"},
		{name: "invalid", id: "", wantErr: ErrInvalidInput},
		{name: "missing", id: "missing", wantErr: ErrNotFound},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			pr, err := svc.Merge(ctx, tt.id)
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			require.Equal(t, "MERGED", pr.Status)
			require.NotNil(t, pr.MergedAt)
		})
	}
}

func TestReassign(t *testing.T) {
	ctx := context.Background()
	tests := []struct {
		name    string
		id      string
		old     string
		wantErr error
	}{
		{name: "ok", id: "pr1", old: "b"},
		{name: "invalid", id: "", old: "", wantErr: ErrInvalidInput},
		{name: "missing pr", id: "missing", old: "b", wantErr: ErrNotFound},
		{name: "merged", id: "pr2", old: "b", wantErr: ErrMerged},
		{name: "not assigned", id: "pr3", old: "c", wantErr: ErrNotAssigned},
		{name: "no candidate", id: "pr4", old: "b", wantErr: ErrNoCandidate},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			repo := newPRRepoStub()
			repo.users["a"] = entity.User{UserID: "a", TeamName: "team", IsActive: true}
			repo.users["b"] = entity.User{UserID: "b", TeamName: "team", IsActive: true}
			repo.users["c"] = entity.User{UserID: "c", TeamName: "team", IsActive: true}
			repo.prs["pr1"] = entity.PullRequest{PullRequestID: "pr1", AuthorID: "a", Status: "OPEN"}
			repo.reviewers["pr1"] = []string{"b"}
			repo.prs["pr2"] = entity.PullRequest{PullRequestID: "pr2", Status: "MERGED"}
			repo.reviewers["pr2"] = []string{"b"}
			repo.prs["pr3"] = entity.PullRequest{PullRequestID: "pr3", Status: "OPEN"}
			repo.reviewers["pr3"] = []string{"b"}
			repo.prs["pr4"] = entity.PullRequest{PullRequestID: "pr4", AuthorID: "a", Status: "OPEN"}
			repo.reviewers["pr4"] = []string{"b"}
			if tt.name == "no candidate" {
				repo.users["c"] = entity.User{UserID: "c", TeamName: "team", IsActive: false}
			}

			svc := NewService(repo)
			svc.rand = randSource(2)
			pr, replaced, err := svc.Reassign(ctx, tt.id, tt.old)
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			require.NotEqual(t, tt.old, replaced)
			require.Contains(t, pr.Assigned, replaced)
			require.NotContains(t, pr.Assigned, tt.old)
		})
	}
}

func randSource(seed int64) *rand.Rand {
	return rand.New(rand.NewSource(seed))
}
