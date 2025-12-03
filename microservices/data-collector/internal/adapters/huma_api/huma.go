package huma_api

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/DanieleT25/FlightData-Manager/microservices/data-collector/internal/application/core/apperrors"
	"github.com/DanieleT25/FlightData-Manager/microservices/data-collector/internal/application/core/domain"
	"github.com/DanieleT25/FlightData-Manager/microservices/data-collector/internal/ports"
	"github.com/danielgtaylor/huma/v2"
)

type APIHandler struct {
	app ports.CollectorAPI
}

func NewAPIHandler(app ports.CollectorAPI) *APIHandler {
	return &APIHandler{app: app}
}

type SetInterestsInput struct {
	Body struct {
		UserEmail    string   `json:"user_email" doc:"User email" example:"mario.rossi@email.it" required:"true"`
		Password     string   `json:"password" doc:"User password for verification" required:"true"`
		AirportCodes []string `json:"airport_codes" doc:"List of ICAO codes" example:"[\"LICC\", \"LIRF\"]" required:"true" minItems:"1"`
	}
}

type AuthQueryInput struct {
	UserEmail string `query:"email" doc:"User email" required:"true"`
	Password  string `query:"password" doc:"User password" required:"true"`
}

type FlightRequestInput struct {
	AuthQueryInput
	AirportCode string `path:"code" doc:"ICAO Airport Code" example:"LICC"`
	Direction   string `query:"direction" doc:"'arrival' or 'departure'" default:"departure"`
	Limit       int    `query:"limit" doc:"Max number of flights to return" default:"10"`
}

type StatsRequestInput struct {
	AuthQueryInput
	AirportCode string `path:"code"`
	Direction   string `query:"direction" default:"departure"`
	Days        int    `query:"days" default:"7"`
}

type SetInterestsOutput struct {
	Body struct {
		Message string `json:"message" example:"Interests updated successfully"`
	}
}

type InterestsOutput struct {
	Body struct {
		AirportCodes []string `json:"tracked_airports"`
	}
}

type SingleFlightOutput struct {
	Body struct {
		Flight FlightResponse `json:"flight"`
	}
}

type FlightListOutput struct {
	Body struct {
		Flights []FlightResponse `json:"flights"`
	}
}

type StatsOutput struct {
	Body struct {
		Airport        string  `json:"airport"`
		Direction      string  `json:"direction"`
		AverageFlights float64 `json:"average_daily_flights"`
	}
}

type FlightResponse struct {
	ICAO24              string    `json:"icao24"`
	FirstSeen           time.Time `json:"firstSeen" doc:"Departure time (ISO 8601)"`
	LastSeen            time.Time `json:"lastSeen" doc:"Arrival time (ISO 8601)"`
	EstDepartureAirport string    `json:"estDepartureAirport"`
	EstArrivalAirport   string    `json:"estArrivalAirport"`
	Callsign            string    `json:"callsign"`
	Type                string    `json:"type"`
}

func mapToResponse(f *domain.Flight) FlightResponse {
	return FlightResponse{
		ICAO24:              f.ICAO24,
		FirstSeen:           time.Unix(f.FirstSeen, 0).UTC(),
		LastSeen:            time.Unix(f.LastSeen, 0).UTC(),
		EstDepartureAirport: f.EstDepartureAirport,
		EstArrivalAirport:   f.EstArrivalAirport,
		Callsign:            f.Callsign,
		Type:                f.Type,
	}
}

func (h *APIHandler) SetInterestsHandler(ctx context.Context, input *SetInterestsInput) (*SetInterestsOutput, error) {
	err := h.app.SetUserInterests(ctx, input.Body.UserEmail, input.Body.Password, input.Body.AirportCodes)
	if err != nil {
		return nil, mapError(err)
	}
	resp := &SetInterestsOutput{}
	resp.Body.Message = "Interests updated successfully"

	return resp, nil
}

func (h *APIHandler) GetUserInterestsHandler(ctx context.Context, input *AuthQueryInput) (*InterestsOutput, error) {
	codes, err := h.app.GetUserInterests(ctx, input.UserEmail, input.Password)
	if err != nil {
		return nil, mapError(err)
	}

	resp := &InterestsOutput{}
	resp.Body.AirportCodes = codes
	return resp, nil
}

func (h *APIHandler) GetAirportFlightsHandler(ctx context.Context, input *FlightRequestInput) (*FlightListOutput, error) {
	flights, err := h.app.GetAirportFlights(ctx, input.UserEmail, input.Password, input.AirportCode, input.Limit)
	if err != nil {
		return nil, mapError(err)
	}

	if len(flights) == 0 {
		return nil, huma.NewError(http.StatusNotFound, "No data available yet. Please retry later.")
	}

	resp := &FlightListOutput{}
	dtos := make([]FlightResponse, len(flights))

	for i, f := range flights {
		dtos[i] = mapToResponse(&f)
	}

	resp.Body.Flights = dtos
	return resp, nil
}

func (h *APIHandler) GetLastFlightHandler(ctx context.Context, input *FlightRequestInput) (*SingleFlightOutput, error) {
	flight, err := h.app.GetLastFlight(ctx, input.UserEmail, input.Password, input.AirportCode, input.Direction)
	if err != nil {
		return nil, mapError(err)
	}

	resp := &SingleFlightOutput{}
	resp.Body.Flight = mapToResponse(flight)
	return resp, nil
}

func (h *APIHandler) GetAverageHandler(ctx context.Context, input *StatsRequestInput) (*StatsOutput, error) {
	avg, err := h.app.GetFlightsAverage(ctx, input.UserEmail, input.Password, input.AirportCode, input.Direction, input.Days)
	if err != nil {
		return nil, mapError(err)
	}

	resp := &StatsOutput{}
	resp.Body.Airport = input.AirportCode
	resp.Body.Direction = input.Direction
	resp.Body.AverageFlights = avg
	return resp, nil
}

func mapError(err error) error {
	switch {
	case errors.Is(err, apperrors.ErrInvalidInput):
		return huma.Error400BadRequest(err.Error())
	case errors.Is(err, apperrors.ErrUserNotAuthorized):
		return huma.Error401Unauthorized("Invalid credentials or unauthorized access")
	case errors.Is(err, apperrors.ErrUserNotFound):
		return huma.Error404NotFound("User not found")
	case errors.Is(err, apperrors.ErrNoDataFound):
		return huma.NewError(http.StatusNotFound, "No data available yet. Monitoring cycle runs periodically (approx. every 12h). Please retry later.")
	case errors.Is(err, apperrors.ErrExternalService):
		return huma.Error503ServiceUnavailable("External service unavailable")
	default:
		return huma.Error500InternalServerError("Internal server error", err)
	}
}

func (h *APIHandler) RegisterRoutes(api huma.API) {

	huma.Register(api, huma.Operation{
		OperationID: "set-interests",
		Method:      http.MethodPost,
		Path:        "/interests",
		Summary:     "Set user interests",
		Description: "Updates the list of airports monitored by the user. Requires password for identity verification.",
		Tags:        []string{"Interests"},
	}, h.SetInterestsHandler)

	huma.Register(api, huma.Operation{
		OperationID: "get-interests",
		Method:      http.MethodGet,
		Path:        "/interests",
		Summary:     "Get user interests",
		Description: "Retrieves the list of airport codes currently monitored by the user. Requires password for identity verification.",
		Tags:        []string{"Interests"},
	}, h.GetUserInterestsHandler)

	huma.Register(api, huma.Operation{
		OperationID: "get-airport-flights",
		Method:      http.MethodGet,
		Path:        "/airports/{code}/flights",
		Summary:     "Get flight list",
		Description: "Retrieves a historical list of flights for a specific airport. Requires password for identity verification.",
		Tags:        []string{"Data"},
	}, h.GetAirportFlightsHandler)

	huma.Register(api, huma.Operation{
		OperationID: "get-last-flight",
		Method:      http.MethodGet,
		Path:        "/airports/{code}/flights/last",
		Summary:     "Get last flight",
		Description: "Retrieves the most recent flight (arrival or departure) recorded for a specific airport. Requires password for identity verification.",
		Tags:        []string{"Data"},
	}, h.GetLastFlightHandler)

	huma.Register(api, huma.Operation{
		OperationID: "get-average-stats",
		Method:      http.MethodGet,
		Path:        "/airports/{code}/stats/average",
		Summary:     "Get flight average",
		Description: "Calculates the average number of daily flights for a specific airport over a specified period. Requires password for identity verification.",
		Tags:        []string{"Analytics"},
	}, h.GetAverageHandler)
}
