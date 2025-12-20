package opensky

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/DanieleT25/FlightData-Manager/microservices/data-collector/internal/application/core/apperrors"
	"github.com/DanieleT25/FlightData-Manager/microservices/data-collector/internal/application/core/domain"
)

const (
	// apiBaseURL = "https://pippo.xyz/api"
	apiBaseURL = "https://opensky-network.org/api"
	authURL    = "https://auth.opensky-network.org/auth/realms/opensky-network/protocol/openid-connect/token"
)

type OpenSkyClient struct {
	clientID     string
	clientSecret string
	client       *http.Client
	token        string
	cb           *CircuitBreaker
}

type authResponse struct {
	AccessToken string `json:"access_token"`
}

func NewOpenSkyClient(clientID, clientSecret string) *OpenSkyClient {
	cb := NewCircuitBreaker(3, 30*time.Second)

	return &OpenSkyClient{
		clientID:     clientID,
		clientSecret: clientSecret,
		client:       &http.Client{Timeout: 60 * time.Second},
		cb:           cb,
	}
}

func (o *OpenSkyClient) authenticate(ctx context.Context) error {
	data := url.Values{}
	data.Set("grant_type", "client_credentials")
	data.Set("client_id", o.clientID)
	data.Set("client_secret", o.clientSecret)

	req, err := http.NewRequestWithContext(ctx, "POST", authURL, strings.NewReader(data.Encode()))
	if err != nil {
		return fmt.Errorf("failed to create auth request: %w", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := o.client.Do(req)
	if err != nil {
		return fmt.Errorf("auth request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("auth failed with status %d", resp.StatusCode)
	}

	var authResp authResponse
	if err := json.NewDecoder(resp.Body).Decode(&authResp); err != nil {
		return fmt.Errorf("failed to decode auth token: %w", err)
	}

	o.token = authResp.AccessToken
	return nil
}

func (o *OpenSkyClient) fetchFlights(ctx context.Context, apiURL string, flightType string) ([]domain.Flight, error) {
	executeRequest := func() (any, error) {
		req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
		if err != nil {
			return nil, err
		}

		if o.clientID != "" {
			if o.token == "" {
				if err := o.authenticate(ctx); err != nil {
					return nil, err
				}
			}
			req.Header.Set("Authorization", "Bearer "+o.token)
		}

		resp, err := o.client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("%w: request failed: %v", apperrors.ErrExternalService, err)
		}

		if resp.StatusCode == http.StatusUnauthorized && o.clientID != "" {
			resp.Body.Close()
			if err := o.authenticate(ctx); err != nil {
				return nil, fmt.Errorf("%w: token refresh failed: %v", apperrors.ErrExternalService, err)
			}
			req.Header.Set("Authorization", "Bearer "+o.token)
			resp, err = o.client.Do(req)
			if err != nil {
				return nil, fmt.Errorf("%w: retry request failed: %v", apperrors.ErrExternalService, err)
			}
		}

		defer resp.Body.Close()

		if resp.StatusCode == http.StatusNotFound {
			return []domain.Flight{}, nil
		}

		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("%w: opensky returned status %d", apperrors.ErrExternalService, resp.StatusCode)
		}

		var flights []domain.Flight
		if err := json.NewDecoder(resp.Body).Decode(&flights); err != nil {
			return nil, fmt.Errorf("%w: failed to decode json: %v", apperrors.ErrExternalService, err)
		}

		return flights, nil
	}

	result, err := o.cb.Execute(executeRequest)

	if err != nil {
		if err == ErrCircuitOpen {
			return nil, fmt.Errorf("%w: circuit breaker open", apperrors.ErrExternalService)
		}
		return nil, err
	}

	flights, ok := result.([]domain.Flight)
	if !ok {
		return nil, fmt.Errorf("internal error: unexpected response type")
	}

	for i := range flights {
		flights[i].Type = flightType
	}

	return flights, nil
}

func (o *OpenSkyClient) GetArrivals(ctx context.Context, airport string, begin, end int64) ([]domain.Flight, error) {
	url := fmt.Sprintf("%s/flights/arrival?airport=%s&begin=%d&end=%d", apiBaseURL, airport, begin, end)
	return o.fetchFlights(ctx, url, "arrival")
}

func (o *OpenSkyClient) GetDepartures(ctx context.Context, airport string, begin, end int64) ([]domain.Flight, error) {
	url := fmt.Sprintf("%s/flights/departure?airport=%s&begin=%d&end=%d", apiBaseURL, airport, begin, end)
	return o.fetchFlights(ctx, url, "departure")
}
