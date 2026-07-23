package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/DanieleT25/FlightData-Manager/microservices/user-manager/internal/application/core/apperrors"
	"github.com/DanieleT25/FlightData-Manager/microservices/user-manager/internal/application/core/domain"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

const uniqueViolationCode = "23505"

const createUsersTableSQL = `
CREATE TABLE IF NOT EXISTS users (
	email           TEXT PRIMARY KEY,
	first_name      TEXT NOT NULL,
	last_name       TEXT NOT NULL,
	password_hash   TEXT NOT NULL,
	card_number     TEXT NOT NULL,
	expiration_date TEXT NOT NULL,
	cvv             TEXT NOT NULL,
	registered_at   TIMESTAMPTZ NOT NULL
)`

type PostgresRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresRepository(ctx context.Context, dsn string) (*PostgresRepository, error) {
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to create postgres pool: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("failed to connect to postgres: %w", err)
	}

	if _, err := pool.Exec(ctx, createUsersTableSQL); err != nil {
		pool.Close()
		return nil, fmt.Errorf("failed to create users table: %w", err)
	}

	return &PostgresRepository{pool: pool}, nil
}

func (r *PostgresRepository) Save(ctx context.Context, user *domain.User) error {
	const query = `
		INSERT INTO users (email, first_name, last_name, password_hash, card_number, expiration_date, cvv, registered_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`

	_, err := r.pool.Exec(ctx, query,
		user.Email,
		user.FirstName,
		user.LastName,
		user.PasswordHash,
		user.BankingDetails.CardNumber,
		user.BankingDetails.ExpirationDate,
		user.BankingDetails.CVV,
		user.RegisteredAt,
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == uniqueViolationCode {
			return apperrors.ErrUserAlreadyExists
		}
		return fmt.Errorf("failed to save user: %w", err)
	}

	return nil
}

func (r *PostgresRepository) Get(ctx context.Context, email string) (*domain.User, error) {
	const query = `
		SELECT email, first_name, last_name, password_hash, card_number, expiration_date, cvv, registered_at
		FROM users WHERE email = $1`

	var user domain.User
	err := r.pool.QueryRow(ctx, query, email).Scan(
		&user.Email,
		&user.FirstName,
		&user.LastName,
		&user.PasswordHash,
		&user.BankingDetails.CardNumber,
		&user.BankingDetails.ExpirationDate,
		&user.BankingDetails.CVV,
		&user.RegisteredAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.ErrUserNotFound
		}
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	return &user, nil
}

func (r *PostgresRepository) Delete(ctx context.Context, email string) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM users WHERE email = $1`, email)
	if err != nil {
		return fmt.Errorf("failed to delete user: %w", err)
	}

	if tag.RowsAffected() == 0 {
		return apperrors.ErrUserNotFound
	}

	return nil
}
