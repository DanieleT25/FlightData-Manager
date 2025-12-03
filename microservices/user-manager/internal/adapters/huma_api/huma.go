package huma_api

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/DanieleT25/FlightData-Manager/microservices/user-manager/internal/application/core/apperrors"
	"github.com/DanieleT25/FlightData-Manager/microservices/user-manager/internal/application/core/domain"
	"github.com/DanieleT25/FlightData-Manager/microservices/user-manager/internal/middleware"
	"github.com/DanieleT25/FlightData-Manager/microservices/user-manager/internal/ports"
	"github.com/danielgtaylor/huma/v2"
)

type APIHandler struct {
	app ports.UserAPI
}

func NewAPIHandler(app ports.UserAPI) *APIHandler {
	return &APIHandler{
		app: app,
	}
}

type RegisterInput struct {
	RequestID string `header:"X-Request-ID" doc:"Unique request identifier (UUID)" required:"true"`
	Body      struct {
		Email          string `json:"email" doc:"User's email address (Primary Key)" example:"mario.rossi@email.it" format:"email" required:"true"`
		Password       string `json:"password" doc:"User's password" example:"SecureP@ssw0rd!" required:"true"`
		FirstName      string `json:"first_name" doc:"User's name" example:"Mario" required:"true"`
		LastName       string `json:"last_name" doc:"User's surname" example:"Rossi" required:"true"`
		CardNumber     string `json:"card_number" doc:"Credit card number (usually 16 digits)" example:"1234567812345678"`
		ExpirationDate string `json:"expiration_date" doc:"Card expiration date (MM/YY)" example:"12/28"`
		CVV            string `json:"cvv" doc:"Card Security Code (CVV/CVC)" example:"123"`
	}
}

type UserOutput struct {
	Body struct {
		User *domain.User `json:"user"`
	}
}

type EmailInput struct {
	Email string `path:"email" doc:"The email address of the user to retrieve" required:"true"`
}

type DeleteInput struct {
	RequestID string `header:"X-Request-ID" doc:"UUID unique identifier for the request" required:"true"`
	Email     string `path:"email" doc:"The email address of the user to delete" required:"true"`
	Body      struct {
		Password string `json:"password" doc:"Current password for confirmation" required:"true"`
	}
}

func (h *APIHandler) RegisterUserHandler(ctx context.Context, input *RegisterInput) (*UserOutput, error) {
	clientIP := middleware.GetIP(ctx)

	isNew, cachedUser, err := h.app.CheckIdempotencyUser(ctx, clientIP, input.RequestID)
	if err != nil {
		if err.Error() == "request is currently being processed" {
			return nil, huma.Error429TooManyRequests("Request already in progress")
		}
		return nil, mapError(err)
	}

	if !isNew && cachedUser != nil {
		cachedUser.PasswordHash = ""
		resp := &UserOutput{}
		resp.Body.User = cachedUser
		return resp, nil
	}

	user, err := h.app.RegisterUser(
		ctx,
		input.Body.Email,
		input.Body.Password,
		input.Body.FirstName,
		input.Body.LastName,
		input.Body.CardNumber,
		input.Body.ExpirationDate,
		input.Body.CVV,
	)

	if err != nil {
		_ = h.app.DeleteIdempotencyKeyUser(ctx, clientIP, input.RequestID)
		return nil, mapError(err)
	}

	user.PasswordHash = ""
	err = h.app.SaveIdempotencyResponseUser(ctx, clientIP, input.RequestID, user)
	if err != nil {
		fmt.Printf("Warning: failed to save idempotency response: %v\n", err)
	}

	resp := &UserOutput{}
	resp.Body.User = user
	return resp, nil
}

func (h *APIHandler) GetUserHandler(ctx context.Context, input *EmailInput) (*UserOutput, error) {
	user, err := h.app.GetUser(ctx, input.Email)
	if err != nil {
		return nil, mapError(err)
	}

	user.PasswordHash = ""

	resp := &UserOutput{}
	resp.Body.User = user
	return resp, nil
}

func (h *APIHandler) DeleteUserHandler(ctx context.Context, input *DeleteInput) (*struct{}, error) {
	clientIP := middleware.GetIP(ctx)

	isNew, _, err := h.app.CheckIdempotencyUser(ctx, clientIP, input.RequestID)
	if err != nil {
		if err.Error() == "request is currently being processed" {
			return nil, huma.Error429TooManyRequests("Request in progress")
		}
		return nil, mapError(err)
	}

	if !isNew {
		return nil, nil
	}

	err = h.app.DeleteUser(ctx, input.Email, input.Body.Password)
	if err != nil {
		_ = h.app.DeleteIdempotencyKeyUser(ctx, clientIP, input.RequestID)
		return nil, mapError(err)
	}

	err = h.app.SaveIdempotencyResponseUser(ctx, clientIP, input.RequestID, nil)
	if err != nil {
		fmt.Printf("Warning: failed to save idempotency response: %v\n", err)
	}
	return nil, nil
}

func mapError(err error) error {
	switch {
	case errors.Is(err, apperrors.ErrInvalidInput):
		return huma.Error400BadRequest(err.Error())
	case errors.Is(err, apperrors.ErrUserAlreadyExists):
		return huma.Error409Conflict("User already exists")
	case errors.Is(err, apperrors.ErrUserNotFound):
		return huma.Error404NotFound("User not found")
	case errors.Is(err, apperrors.ErrInvalidPassword):
		return huma.Error401Unauthorized("Invalid password")
	default:
		return huma.Error500InternalServerError("Internal server error", err)
	}
}

func (h *APIHandler) RegisterRoutes(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "register-user",
		Method:      http.MethodPost,
		Path:        "/users",
		Summary:     "Register a new user",
		Description: "Creates a new user enforcing an at-most-once policy.",
		Tags:        []string{"Users"},
	}, h.RegisterUserHandler)

	huma.Register(api, huma.Operation{
		OperationID: "get-user",
		Method:      http.MethodGet,
		Path:        "/users/{email}",
		Summary:     "Get user details",
		Description: "Get all user information (except password).",
		Tags:        []string{"Users"},
	}, h.GetUserHandler)

	huma.Register(api, huma.Operation{
		OperationID: "delete-user",
		Method:      http.MethodDelete,
		Path:        "/users/{email}",
		Summary:     "Delete a user",
		Description: "Removes a user from the system. Requires the password in the request body for identity verification.",
		Tags:        []string{"Users"},
	}, h.DeleteUserHandler)
}
