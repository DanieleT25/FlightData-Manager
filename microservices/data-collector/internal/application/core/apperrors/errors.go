package apperrors

import "errors"

var (
	ErrInvalidInput = errors.New("invalid input data")

	ErrUserNotAuthorized = errors.New("user not authorized to access this airport data")
	ErrUserNotFound      = errors.New("user not found in user manager")

	ErrNoDataFound = errors.New("no data found")
	ErrDbOperation = errors.New("database operation failed")

	ErrExternalService = errors.New("external service unavailable")
)
