package huma_api

import (
	"time"
)

type InterestInputItem struct {
	AirportCode string `json:"airport_code" doc:"ICAO airport code" example:"LICC" required:"true"`
	LowValue    *int   `json:"low_value,omitempty" doc:"Notify if flights < this value" example:"5"`
	HighValue   *int   `json:"high_value,omitempty" doc:"Notify if flights > this value" example:"20"`
}

type AuthQueryInput struct {
	UserEmail string `header:"email" doc:"User email" example:"mario.rossi@email.it" required:"true" format:"email"`
	Password  string `header:"password" doc:"User password" required:"true"`
}

type SetInterestsInput struct {
	AuthQueryInput
	Body struct {
		Interests []InterestInputItem `json:"interests" doc:"List of monitored airports with thresholds" required:"true" minItems:"1"`
	}
}

type FlightRequestInput struct {
	AuthQueryInput
	AirportCode string `path:"code" doc:"ICAO Airport Code" example:"LICC" required:"true"`
	Direction   string `query:"direction" doc:"'arrival' or 'departure'" default:"departure" enum:"arrival,departure"`
	Limit       int    `query:"limit" doc:"Max number of flights to return" default:"10"`
}

type StatsRequestInput struct {
	AuthQueryInput
	AirportCode string `path:"code" doc:"ICAO Airport Code" example:"LICC" required:"true"`
	Direction   string `query:"direction" doc:"'arrival' or 'departure'" default:"departure"`
	Days        int    `query:"days" doc:"Number of days for average calculation" default:"7"`
}

type SetInterestsOutput struct {
	Body struct {
		Message string `json:"message" example:"Interests updated successfully"`
	}
}

type InterestOutputItem struct {
	AirportCode string `json:"airport_code" doc:"ICAO airport code" example:"LICC"`
	LowValue    *int   `json:"low_value,omitempty" doc:"Threshold for low traffic alerts" example:"5"`
	HighValue   *int   `json:"high_value,omitempty" doc:"Threshold for high traffic alerts" example:"20"`
}

type InterestsOutput struct {
	Body struct {
		Interests []InterestOutputItem `json:"tracked_airports" doc:"List of monitored airports with settings"`
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
		Airport        string  `json:"airport" doc:"ICAO Airport Code"`
		Direction      string  `json:"direction" doc:"'arrival' or 'departure'"`
		AverageFlights float64 `json:"average_daily_flights" doc:"Average number of flights"`
	}
}

type FlightResponse struct {
	ICAO24              string    `json:"icao24" doc:"Unique ICAO 24-bit address of the transponder"`
	FirstSeen           time.Time `json:"firstSeen" doc:"Departure time (ISO 8601)"`
	LastSeen            time.Time `json:"lastSeen" doc:"Arrival time (ISO 8601)"`
	EstDepartureAirport string    `json:"estDepartureAirport" doc:"Estimated departure airport ICAO code"`
	EstArrivalAirport   string    `json:"estArrivalAirport" doc:"Estimated arrival airport ICAO code"`
	Callsign            string    `json:"callsign" doc:"Flight callsign"`
	Type                string    `json:"type"`
}
