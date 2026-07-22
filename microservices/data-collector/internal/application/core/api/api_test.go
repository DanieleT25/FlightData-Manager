package api

import (
	"context"
	"testing"
	"time"

	"github.com/DanieleT25/FlightData-Manager/microservices/data-collector/internal/application/core/domain"
)

type fakeFlightRepository struct {
	interests map[string][]domain.Interest
	airports  map[string]int64
	flights   []domain.Flight
	updated   map[string]int64
}

func newFakeFlightRepository() *fakeFlightRepository {
	return &fakeFlightRepository{
		interests: make(map[string][]domain.Interest),
		airports:  make(map[string]int64),
		updated:   make(map[string]int64),
	}
}

func (r *fakeFlightRepository) SetInterests(_ context.Context, email string, interests []domain.Interest) error {
	r.interests[email] = interests
	for _, interest := range interests {
		r.airports[interest.AirportCode] = 0
	}
	return nil
}
func (r *fakeFlightRepository) GetInterests(_ context.Context, email string) ([]domain.Interest, error) {
	return r.interests[email], nil
}
func (r *fakeFlightRepository) IsUserInterested(_ context.Context, email, airport string) (bool, error) {
	for _, interest := range r.interests[email] {
		if interest.AirportCode == airport {
			return true, nil
		}
	}
	return false, nil
}
func (r *fakeFlightRepository) GetFlights(_ context.Context, _ string, _ int) ([]domain.Flight, error) {
	return r.flights, nil
}
func (r *fakeFlightRepository) GetLastFlight(_ context.Context, _ string, _ string) (*domain.Flight, error) {
	if len(r.flights) == 0 {
		return nil, nil
	}
	return &r.flights[len(r.flights)-1], nil
}
func (r *fakeFlightRepository) GetFlightsCount(_ context.Context, _ string, _ string, _ int64) (int64, error) {
	return int64(len(r.flights)), nil
}
func (r *fakeFlightRepository) SetFlight(_ context.Context, flight *domain.Flight) error {
	r.flights = append(r.flights, *flight)
	return nil
}
func (r *fakeFlightRepository) GetAllUsers(_ context.Context) ([]string, error)   { return nil, nil }
func (r *fakeFlightRepository) DeleteUserNodes(_ context.Context, _ string) error { return nil }
func (r *fakeFlightRepository) GetAirportsToMonitor(_ context.Context) (map[string]int64, error) {
	return r.airports, nil
}
func (r *fakeFlightRepository) UpdateAirportLastSync(_ context.Context, airport string, timestamp int64) error {
	r.updated[airport] = timestamp
	return nil
}

type fakeUserManagerClient struct{ valid bool }

func (c fakeUserManagerClient) CheckUserExistence(_ context.Context, _ string) (bool, error) {
	return true, nil
}
func (c fakeUserManagerClient) VerifyCredentials(_ context.Context, _ string, _ string) (bool, error) {
	return c.valid, nil
}

type fakeOpenSkyClient struct{}

func (fakeOpenSkyClient) GetArrivals(_ context.Context, airport string, _, _ int64) ([]domain.Flight, error) {
	return []domain.Flight{{ICAO24: "arrival-1", FirstSeen: 1, LastSeen: 2, EstDepartureAirport: "LIRF", EstArrivalAirport: airport, Callsign: "AZA1", Type: "arrival"}}, nil
}
func (fakeOpenSkyClient) GetDepartures(_ context.Context, airport string, _, _ int64) ([]domain.Flight, error) {
	return []domain.Flight{{ICAO24: "departure-1", FirstSeen: 3, LastSeen: 4, EstDepartureAirport: airport, EstArrivalAirport: "LIRF", Callsign: "AZA2", Type: "departure"}}, nil
}

type capturedEvent struct {
	airport              string
	arrivals, departures int
}
type fakeEventProducer struct{ events []capturedEvent }

func (p *fakeEventProducer) SendUpdate(airport string, arrivals, departures int, _ int64) error {
	p.events = append(p.events, capturedEvent{airport, arrivals, departures})
	return nil
}

func TestSetUserInterestsChecksCredentialsAndPersistsInterests(t *testing.T) {
	repo := newFakeFlightRepository()
	producer := &fakeEventProducer{}
	app := NewApplication(repo, fakeUserManagerClient{valid: true}, fakeOpenSkyClient{}, producer, 12*time.Hour)
	low, high := 5, 20
	interest, _ := domain.NewInterest("mario@example.com", "LICC", &low, &high)

	if err := app.SetUserInterests(context.Background(), "mario@example.com", "Password123!", []domain.Interest{*interest}); err != nil {
		t.Fatalf("SetUserInterests() error = %v", err)
	}
	if got := repo.interests["mario@example.com"]; len(got) != 1 || got[0].AirportCode != "LICC" {
		t.Fatalf("stored interests = %#v", got)
	}
}

func TestRunCollectionCycleStoresFlightsAndEmitsUpdate(t *testing.T) {
	repo := newFakeFlightRepository()
	repo.airports["LICC"] = 0
	producer := &fakeEventProducer{}
	app := NewApplication(repo, fakeUserManagerClient{valid: true}, fakeOpenSkyClient{}, producer, 12*time.Hour)

	if err := app.RunCollectionCycle(context.Background()); err != nil {
		t.Fatalf("RunCollectionCycle() error = %v", err)
	}
	if len(repo.flights) != 2 {
		t.Fatalf("stored flights = %d, want 2", len(repo.flights))
	}
	if len(producer.events) != 1 || producer.events[0].arrivals != 1 || producer.events[0].departures != 1 {
		t.Fatalf("published events = %#v", producer.events)
	}
	if repo.updated["LICC"] == 0 {
		t.Fatal("airport last-sync timestamp was not updated")
	}
}
