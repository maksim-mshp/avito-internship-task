package users

import (
	"context"

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

func (r *Repository) SetIsActive(ctx context.Context, userID string, active bool) (entity.User, error) {
	row := r.db.QueryRow(ctx, `
UPDATE users SET is_active = $2 WHERE user_id = $1
RETURNING user_id, username, team_name, is_active
`, userID, active)
	var u entity.User
	if err := row.Scan(&u.UserID, &u.Username, &u.TeamName, &u.IsActive); err != nil {
		return entity.User{}, err
	}
	return u, nil
}

func (r *Repository) Get(ctx context.Context, userID string) (entity.User, error) {
	row := r.db.QueryRow(ctx, `SELECT user_id, username, team_name, is_active FROM users WHERE user_id = $1`, userID)
	var u entity.User
	if err := row.Scan(&u.UserID, &u.Username, &u.TeamName, &u.IsActive); err != nil {
		return entity.User{}, err
	}
	return u, nil
}

func (r *Repository) GetReview(ctx context.Context, userID string) ([]entity.PullRequestShort, error) {
	rows, err := r.db.Query(ctx, `
SELECT pr.pull_request_id, pr.pull_request_name, pr.author_id, pr.status
FROM pr_reviewers r
JOIN pull_requests pr ON pr.pull_request_id = r.pull_request_id
WHERE r.reviewer_id = $1
`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]entity.PullRequestShort, 0)
	for rows.Next() {
		var pr entity.PullRequestShort
		if err := rows.Scan(&pr.PullRequestID, &pr.PullRequestName, &pr.AuthorID, &pr.Status); err != nil {
			return nil, err
		}
		items = append(items, pr)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

func isNotFound(err error) bool {
	return err != nil && err == pgx.ErrNoRows
}
