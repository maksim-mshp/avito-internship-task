package integration

import (
	"context"
	"fmt"
	"testing"
	"time"

	"avito-internship-task/internal/db"
	"avito-internship-task/internal/entity"
	"avito-internship-task/internal/pullrequests"
	"avito-internship-task/internal/teams"
	"avito-internship-task/internal/users"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	tc "github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

func setupPostgres(t *testing.T) *pgxpool.Pool {
	t.Helper()

	ctx := context.Background()
	user := "postgres"
	password := "postgres"
	dbname := "postgres"

	req := tc.ContainerRequest{
		Image:        "postgres:18-alpine",
		ExposedPorts: []string{"5432/tcp"},
		Env: map[string]string{
			"POSTGRES_USER":     user,
			"POSTGRES_PASSWORD": password,
			"POSTGRES_DB":       dbname,
		},
		WaitingFor: wait.ForListeningPort("5432/tcp").WithStartupTimeout(60 * time.Second),
	}

	container, err := tc.GenericContainer(ctx, tc.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	require.NoError(t, err)

	t.Cleanup(func() {
		_ = container.Terminate(ctx)
	})

	endpoint, err := container.Endpoint(ctx, "")
	require.NoError(t, err)

	dsn := fmt.Sprintf("postgres://%s:%s@%s/%s?sslmode=disable", user, password, endpoint, dbname)
	pool, err := db.Connect(ctx, dsn)
	require.NoError(t, err)

	t.Cleanup(func() {
		pool.Close()
	})

	return pool
}

func TestTeamsRepositoryIntegration(t *testing.T) {
	pool := setupPostgres(t)
	ctx := context.Background()
	repo := teams.NewRepository(pool)

	team := entity.Team{
		TeamName: "backend",
		Members: []entity.TeamMember{
			{UserID: "u1", Username: "Alice", IsActive: true},
			{UserID: "u2", Username: "Bob", IsActive: false},
		},
	}

	require.NoError(t, repo.Create(ctx, team))

	got, err := repo.Get(ctx, "backend")
	require.NoError(t, err)
	require.Equal(t, team.TeamName, got.TeamName)
	require.Len(t, got.Members, 2)
}

func TestUsersRepositoryIntegration(t *testing.T) {
	pool := setupPostgres(t)
	ctx := context.Background()
	teamRepo := teams.NewRepository(pool)
	require.NoError(t, teamRepo.Create(ctx, entity.Team{
		TeamName: "backend",
		Members:  []entity.TeamMember{{UserID: "u1", Username: "Alice", IsActive: true}},
	}))

	userRepo := users.NewRepository(pool)

	user, err := userRepo.SetIsActive(ctx, "u1", false)
	require.NoError(t, err)
	require.False(t, user.IsActive)
	require.Equal(t, "backend", user.TeamName)
}

func TestPullRequestsRepositoryIntegration(t *testing.T) {
	pool := setupPostgres(t)
	ctx := context.Background()
	teamRepo := teams.NewRepository(pool)
	require.NoError(t, teamRepo.Create(ctx, entity.Team{
		TeamName: "backend",
		Members: []entity.TeamMember{
			{UserID: "author", Username: "Author", IsActive: true},
			{UserID: "rev1", Username: "Rev1", IsActive: true},
			{UserID: "rev2", Username: "Rev2", IsActive: true},
		},
	}))

	prRepo := pullrequests.NewRepository(pool)

	pr := entity.PullRequest{
		PullRequestID:   "pr1",
		PullRequestName: "Test",
		AuthorID:        "author",
		Status:          "OPEN",
		Assigned:        []string{"rev1", "rev2"},
	}

	require.NoError(t, prRepo.Create(ctx, pr))

	stored, err := prRepo.Get(ctx, "pr1")
	require.NoError(t, err)
	require.Equal(t, pr.PullRequestID, stored.PullRequestID)
	require.ElementsMatch(t, pr.Assigned, stored.Assigned)

	now := time.Now().UTC()
	require.NoError(t, prRepo.Merge(ctx, "pr1", now))

	merged, err := prRepo.Get(ctx, "pr1")
	require.NoError(t, err)
	require.Equal(t, "MERGED", merged.Status)
	require.NotNil(t, merged.MergedAt)
}
