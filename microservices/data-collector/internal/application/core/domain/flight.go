package domain

import (
	"errors"
	"strings"
)

type Flight struct {
	ICAO24              string `json:"icao24"`
	FirstSeen           int64  `json:"firstSeen"`
	LastSeen            int64  `json:"lastSeen"`
	EstDepartureAirport string `json:"estDepartureAirport"`
	EstArrivalAirport   string `json:"estArrivalAirport"`
	Callsign            string `json:"callsign"`
	Type                string `json:"type"`
}

type Interest struct {
	UserEmail   string `json:"user_email"`
	AirportCode string `json:"airport_code"`
	LowValue    *int   `json:"low_value,omitempty"`
	HighValue   *int   `json:"high_value,omitempty"`
}

func NewInterest(email, airportCode string, low, high *int) (*Interest, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	airportCode = strings.ToUpper(strings.TrimSpace((airportCode)))

	if email == "" {
		return nil, errors.New("emai cannot be empty")
	}
	if airportCode == "" {
		return nil, errors.New("airportCode cannot be empty")
	}
	if len(airportCode) != 4 {
		return nil, errors.New("invalid ICAO airport code (must be 4 chars)")
	}
	if low != nil && high != nil {
		if *low >= *high {
			return nil, errors.New("low-value must be lower than high-value")
		}
	}

	return &Interest{
		UserEmail:   email,
		AirportCode: airportCode,
		LowValue:    low,
		HighValue:   high,
	}, nil
}
