package pullrequests

import (
	"context"
	"time"

	"avito-internship-task/internal/entity"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

func (r *Repository) GetUser(ctx context.Context, userID string) (entity.User, error) {
	row := r.db.QueryRow(ctx, `SELECT user_id, username, team_name, is_active FROM users WHERE user_id = $1`, userID)
	var u entity.User
	if err := row.Scan(&u.UserID, &u.Username, &u.TeamName, &u.IsActive); err != nil {
		return entity.User{}, err
	}
	return u, nil
}

func (r *Repository) GetActiveTeamMembers(ctx context.Context, teamName string) ([]entity.User, error) {
	rows, err := r.db.Query(ctx, `SELECT user_id, username, team_name, is_active FROM users WHERE team_name = $1 AND is_active = TRUE`, teamName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	users := make([]entity.User, 0)
	for rows.Next() {
		var u entity.User
		if err := rows.Scan(&u.UserID, &u.Username, &u.TeamName, &u.IsActive); err != nil {
			return nil, err
		}
		users = append(users, u)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return users, nil
}

func (r *Repository) Create(ctx context.Context, pr entity.PullRequest) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `
INSERT INTO pull_requests (pull_request_id, pull_request_name, author_id, status)
VALUES ($1, $2, $3, $4)
`, pr.PullRequestID, pr.PullRequestName, pr.AuthorID, pr.Status); err != nil {
		return err
	}

	for _, reviewer := range pr.Assigned {
		if _, err := tx.Exec(ctx, `
INSERT INTO pr_reviewers (pull_request_id, reviewer_id) VALUES ($1, $2)
`, pr.PullRequestID, reviewer); err != nil {
			return err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return err
	}
	return nil
}

func (r *Repository) Get(ctx context.Context, id string) (entity.PullRequest, error) {
	row := r.db.QueryRow(ctx, `
SELECT pull_request_id, pull_request_name, author_id, status, created_at, merged_at
FROM pull_requests WHERE pull_request_id = $1
`, id)
	var pr entity.PullRequest
	if err := row.Scan(&pr.PullRequestID, &pr.PullRequestName, &pr.AuthorID, &pr.Status, &pr.CreatedAt, &pr.MergedAt); err != nil {
		return entity.PullRequest{}, err
	}

	revs, err := r.loadReviewers(ctx, id)
	if err != nil {
		return entity.PullRequest{}, err
	}
	pr.Assigned = revs
	return pr, nil
}

func (r *Repository) Merge(ctx context.Context, id string, ts time.Time) error {
	_, err := r.db.Exec(ctx, `
UPDATE pull_requests
SET status = 'MERGED', merged_at = COALESCE(merged_at, $2)
WHERE pull_request_id = $1
`, id, ts)
	return err
}

func (r *Repository) ReplaceReviewer(ctx context.Context, prID, oldID, newID string) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `DELETE FROM pr_reviewers WHERE pull_request_id = $1 AND reviewer_id = $2`, prID, oldID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO pr_reviewers (pull_request_id, reviewer_id) VALUES ($1, $2)`, prID, newID); err != nil {
		return err
	}

	return tx.Commit(ctx)
}

func (r *Repository) loadReviewers(ctx context.Context, prID string) ([]string, error) {
	rows, err := r.db.Query(ctx, `SELECT reviewer_id FROM pr_reviewers WHERE pull_request_id = $1`, prID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]string, 0)
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		result = append(result, id)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return result, nil
}

func isNotFound(err error) bool {
	return err != nil && err == pgx.ErrNoRows
}

func (r *Repository) StatsAssignments(ctx context.Context) (map[string]int, error) {
	rows, err := r.db.Query(ctx, `SELECT reviewer_id, COUNT(*) FROM pr_reviewers GROUP BY reviewer_id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	stats := make(map[string]int)
	for rows.Next() {
		var id string
		var cnt int
		if err := rows.Scan(&id, &cnt); err != nil {
			return nil, err
		}
		stats[id] = cnt
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return stats, nil
}
