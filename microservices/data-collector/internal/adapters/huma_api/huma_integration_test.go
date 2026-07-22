package huma_api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/DanieleT25/FlightData-Manager/microservices/data-collector/internal/application/core/domain"
	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humago"
)

type httpCollectorApplication struct {
	interests []domain.Interest
}

func (a *httpCollectorApplication) SetUserInterests(_ context.Context, _, _ string, interests []domain.Interest) error {
	a.interests = interests
	return nil
}
func (a *httpCollectorApplication) GetUserInterests(_ context.Context, _, _ string) ([]domain.Interest, error) {
	return a.interests, nil
}
func (*httpCollectorApplication) GetAirportFlights(context.Context, string, string, string, int) ([]domain.Flight, error) {
	return nil, nil
}
func (*httpCollectorApplication) GetLastFlight(context.Context, string, string, string, string) (*domain.Flight, error) {
	return nil, nil
}
func (*httpCollectorApplication) GetFlightsAverage(context.Context, string, string, string, string, int) (float64, error) {
	return 0, nil
}
func (*httpCollectorApplication) RunCollectionCycle(context.Context) error { return nil }

func TestSetInterestsRouteIntegratesHTTPAdapter(t *testing.T) {
	app := &httpCollectorApplication{}
	mux := http.NewServeMux()
	api := humago.New(mux, huma.DefaultConfig("test", "1.0.0"))
	NewAPIHandler(app).RegisterRoutes(api)

	req := httptest.NewRequest(http.MethodPost, "/interests", strings.NewReader(`{"interests":[{"airport_code":"licc","low_value":5,"high_value":20}]}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("email", "mario@example.com")
	req.Header.Set("password", "Password123!")
	res := httptest.NewRecorder()
	mux.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("POST /interests status = %d, body = %s", res.Code, res.Body.String())
	}
	if len(app.interests) != 1 || app.interests[0].AirportCode != "LICC" {
		t.Fatalf("interests passed to application = %#v", app.interests)
	}
}
