package teams

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

func (r *Repository) Create(ctx context.Context, team entity.Team) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `INSERT INTO teams (name) VALUES ($1)`, team.TeamName); err != nil {
		return err
	}

	for _, m := range team.Members {
		if _, err := tx.Exec(
			ctx,
			`INSERT INTO users (user_id, username, team_name, is_active) VALUES ($1, $2, $3, $4)`,
			m.UserID,
			m.Username,
			team.TeamName,
			m.IsActive,
		); err != nil {
			return err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return err
	}
	return nil
}

func (r *Repository) Get(ctx context.Context, name string) (entity.Team, error) {
	row := r.db.QueryRow(ctx, `SELECT name FROM teams WHERE name = $1`, name)
	var teamName string
	if err := row.Scan(&teamName); err != nil {
		return entity.Team{}, err
	}

	rows, err := r.db.Query(ctx, `SELECT user_id, username, is_active FROM users WHERE team_name = $1`, name)
	if err != nil {
		return entity.Team{}, err
	}
	defer rows.Close()

	members := make([]entity.TeamMember, 0)
	for rows.Next() {
		var m entity.TeamMember
		if err := rows.Scan(&m.UserID, &m.Username, &m.IsActive); err != nil {
			return entity.Team{}, err
		}
		members = append(members, m)
	}
	if err := rows.Err(); err != nil {
		return entity.Team{}, err
	}

	return entity.Team{TeamName: teamName, Members: members}, nil
}

func isNotFound(err error) bool {
	return err != nil && err == pgx.ErrNoRows
}
