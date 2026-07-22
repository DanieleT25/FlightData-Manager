package api

import (
	"context"
	"errors"
	"testing"

	"github.com/DanieleT25/FlightData-Manager/microservices/user-manager/internal/application/core/apperrors"
	"github.com/DanieleT25/FlightData-Manager/microservices/user-manager/internal/application/core/domain"
)

type memoryUserRepository struct {
	users       map[string]*domain.User
	idempotency map[string]*domain.User
}

func newMemoryUserRepository() *memoryUserRepository {
	return &memoryUserRepository{
		users:       make(map[string]*domain.User),
		idempotency: make(map[string]*domain.User),
	}
}

func (r *memoryUserRepository) Save(_ context.Context, user *domain.User) error {
	if _, found := r.users[user.Email]; found {
		return apperrors.ErrUserAlreadyExists
	}
	r.users[user.Email] = user
	return nil
}

func (r *memoryUserRepository) Get(_ context.Context, email string) (*domain.User, error) {
	user, found := r.users[email]
	if !found {
		return nil, apperrors.ErrUserNotFound
	}
	return user, nil
}

func (r *memoryUserRepository) Delete(_ context.Context, email string) error {
	if _, found := r.users[email]; !found {
		return apperrors.ErrUserNotFound
	}
	delete(r.users, email)
	return nil
}

func (r *memoryUserRepository) CheckIdempotency(_ context.Context, clientIP, messageID string) (bool, *domain.User, error) {
	key := clientIP + ":" + messageID
	user, found := r.idempotency[key]
	if !found {
		return true, nil, nil
	}
	return false, user, nil
}

func (r *memoryUserRepository) SaveIdempotencyResponse(_ context.Context, clientIP, messageID string, user *domain.User) error {
	r.idempotency[clientIP+":"+messageID] = user
	return nil
}

func (r *memoryUserRepository) DeleteIdempotencyKey(_ context.Context, clientIP, messageID string) error {
	delete(r.idempotency, clientIP+":"+messageID)
	return nil
}

func TestApplicationRegistersAndDeletesUser(t *testing.T) {
	repo := newMemoryUserRepository()
	app := NewApplication(repo)
	ctx := context.Background()

	user, err := app.RegisterUser(ctx, "MARIO@EXAMPLE.COM", "Password123!", "Mario", "Rossi", "1234", "12/30", "123")
	if err != nil {
		t.Fatalf("RegisterUser() error = %v", err)
	}
	if user.Email != "mario@example.com" || user.PasswordHash == "" {
		t.Fatalf("RegisterUser() user = %+v", user)
	}

	if _, err := app.RegisterUser(ctx, "mario@example.com", "Password123!", "Mario", "Rossi", "1234", "12/30", "123"); !errors.Is(err, apperrors.ErrUserAlreadyExists) {
		t.Fatalf("duplicate RegisterUser() error = %v, want ErrUserAlreadyExists", err)
	}

	if err := app.DeleteUser(ctx, "MARIO@EXAMPLE.COM", "Password123!"); err != nil {
		t.Fatalf("DeleteUser() error = %v", err)
	}
	if _, err := app.GetUser(ctx, "mario@example.com"); !errors.Is(err, apperrors.ErrUserNotFound) {
		t.Fatalf("GetUser() after deletion error = %v, want ErrUserNotFound", err)
	}
}

func TestApplicationRejectsWrongPassword(t *testing.T) {
	repo := newMemoryUserRepository()
	app := NewApplication(repo)
	ctx := context.Background()

	if _, err := app.RegisterUser(ctx, "mario@example.com", "Password123!", "Mario", "Rossi", "1234", "12/30", "123"); err != nil {
		t.Fatalf("RegisterUser() error = %v", err)
	}

	if err := app.DeleteUser(ctx, "mario@example.com", "wrong-password"); !errors.Is(err, apperrors.ErrInvalidPassword) {
		t.Fatalf("DeleteUser() error = %v, want ErrInvalidPassword", err)
	}
}
