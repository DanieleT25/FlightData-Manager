package huma_api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/DanieleT25/FlightData-Manager/microservices/user-manager/internal/application/core/apperrors"
	"github.com/DanieleT25/FlightData-Manager/microservices/user-manager/internal/application/core/domain"
	"github.com/DanieleT25/FlightData-Manager/microservices/user-manager/internal/middleware"
	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humago"
)

type httpUserApplication struct {
	user *domain.User
}

func (a *httpUserApplication) RegisterUser(_ context.Context, email, _ string, firstName, lastName, cardNum, expDate, cvv string) (*domain.User, error) {
	a.user = &domain.User{Email: email, FirstName: firstName, LastName: lastName, PasswordHash: "hash", BankingDetails: domain.CardDetails{CardNumber: cardNum, ExpirationDate: expDate, CVV: cvv}}
	return a.user, nil
}

func (a *httpUserApplication) GetUser(_ context.Context, _ string) (*domain.User, error) {
	if a.user == nil {
		return nil, apperrors.ErrUserNotFound
	}
	return a.user, nil
}

func (a *httpUserApplication) DeleteUser(_ context.Context, _, _ string) error { return nil }
func (a *httpUserApplication) CheckIdempotencyUser(_ context.Context, _, _ string) (bool, *domain.User, error) {
	return true, nil, nil
}
func (a *httpUserApplication) SaveIdempotencyResponseUser(_ context.Context, _, _ string, _ *domain.User) error {
	return nil
}
func (a *httpUserApplication) DeleteIdempotencyKeyUser(_ context.Context, _, _ string) error {
	return nil
}

func TestRegisterRouteIntegratesHTTPAdapter(t *testing.T) {
	app := &httpUserApplication{}
	mux := http.NewServeMux()
	api := humago.New(mux, huma.DefaultConfig("test", "1.0.0"))
	NewAPIHandler(app).RegisterRoutes(api)

	req := httptest.NewRequest(http.MethodPost, "/users", strings.NewReader(`{"email":"mario@example.com","password":"Password123!","first_name":"Mario","last_name":"Rossi","card_number":"1234","expiration_date":"12/30","cvv":"123"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Request-ID", "request-1")
	req.Header.Set("X-Forwarded-For", "203.0.113.1")
	res := httptest.NewRecorder()
	middleware.IPMiddleware(mux).ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("POST /users status = %d, body = %s", res.Code, res.Body.String())
	}
	if !strings.Contains(res.Body.String(), `"email":"mario@example.com"`) {
		t.Fatalf("POST /users response = %s", res.Body.String())
	}
}
