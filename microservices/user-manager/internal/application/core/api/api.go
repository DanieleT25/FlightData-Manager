package api

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/DanieleT25/FlightData-Manager/microservices/user-manager/internal/application/core/apperrors"
	"github.com/DanieleT25/FlightData-Manager/microservices/user-manager/internal/application/core/domain"
	"github.com/DanieleT25/FlightData-Manager/microservices/user-manager/internal/ports"
	"golang.org/x/crypto/bcrypt"
)

type Application struct {
	db    ports.UserRepository
	cache ports.IdempotencyRepository
}

func NewApplication(db ports.UserRepository, cache ports.IdempotencyRepository) *Application {
	return &Application{
		db:    db,
		cache: cache,
	}
}

func (a *Application) RegisterUser(ctx context.Context, email, password, firstName, lastName, cardNum, expDate, cvv string) (*domain.User, error) {
	hashedBytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}

	passwordHash := string(hashedBytes)
	newUser, err := domain.NewUser(firstName, lastName, email, passwordHash, cardNum, expDate, cvv)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", apperrors.ErrInvalidInput, err)
	}

	err = a.db.Save(ctx, newUser)
	if err != nil {
		if errors.Is(err, apperrors.ErrUserAlreadyExists) {
			return nil, apperrors.ErrUserAlreadyExists
		}
		return nil, fmt.Errorf("failed to save user: %w", err)
	}

	return newUser, nil
}

func (a *Application) GetUser(ctx context.Context, email string) (*domain.User, error) {
	email = strings.ToLower(email)
	return a.db.Get(ctx, email)
}

func (a *Application) DeleteUser(ctx context.Context, email string, password string) error {
	email = strings.ToLower(email)

	user, err := a.GetUser(ctx, email)
	if err != nil {
		if errors.Is(err, apperrors.ErrUserNotFound) {
			return apperrors.ErrUserNotFound
		}
		return fmt.Errorf("failed to get user: %w", err)
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return apperrors.ErrInvalidPassword
	}

	return a.db.Delete(ctx, email)
}

func (a *Application) CheckIdempotencyUser(ctx context.Context, ip, msgID string) (bool, *domain.User, error) {
	return a.cache.CheckIdempotency(ctx, ip, msgID)
}

func (a *Application) SaveIdempotencyResponseUser(ctx context.Context, ip, mesgID string, user *domain.User) error {
	return a.cache.SaveIdempotencyResponse(ctx, ip, mesgID, user)
}

func (a *Application) DeleteIdempotencyKeyUser(ctx context.Context, ip, msgID string) error {
	return a.cache.DeleteIdempotencyKey(ctx, ip, msgID)
}
