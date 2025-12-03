package api

import (
	"context"
	"fmt"

	"github.com/DanieleT25/FlightData-Manager/microservices/data-collector/internal/application/core/apperrors"
)

func (a *Application) verifyUserCredentials(ctx context.Context, email, password string) error {
	valid, err := a.userClient.VerifyCredentials(ctx, email, password)

	if err != nil {
		return fmt.Errorf("%w: %v", apperrors.ErrExternalService, err)
	}

	if !valid {
		return apperrors.ErrUserNotAuthorized
	}

	return nil
}

func (a *Application) checkAccessWithPassword(ctx context.Context, email, password, airportCode string) error {
	if err := a.verifyUserCredentials(ctx, email, password); err != nil {
		return err
	}

	allowed, err := a.db.IsUserInterested(ctx, email, airportCode)
	if err != nil {
		return fmt.Errorf("%w: %v", apperrors.ErrDbOperation, err)
	}
	if !allowed {
		return apperrors.ErrUserNotAuthorized
	}

	return nil
}
