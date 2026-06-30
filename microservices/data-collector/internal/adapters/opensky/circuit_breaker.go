package opensky

import (
	"errors"
	"sync"
	"time"

	"github.com/DanieleT25/FlightData-Manager/microservices/data-collector/internal/adapters/observability"
)

type State int

const (
	StateClosed State = iota
	StateOpen
	StateHalfOpen
)

var ErrCircuitOpen = errors.New("circuit is open")

type CircuitBreaker struct {
	mu               sync.Mutex
	state            State
	failureThreshold int
	recoveryTimeout  time.Duration
	failureCount     int
	lastFailureTime  time.Time
	monitor          *observability.Monitor
	targetName       string
}

func NewCircuitBreaker(threshold int, recoveryTimeout time.Duration, monitor *observability.Monitor) *CircuitBreaker {
	cb := &CircuitBreaker{
		state:            StateClosed,
		failureThreshold: threshold,
		recoveryTimeout:  recoveryTimeout,
		monitor:          monitor,
		targetName:       "opensky",
	}

	if monitor != nil {
		monitor.SetCBState(cb.targetName, 0)
	}

	return cb
}

func (cb *CircuitBreaker) changeState(newState State) {
	if cb.state == newState {
		return
	}
	cb.state = newState

	if cb.monitor != nil {
		var stateCode float64
		switch newState {
		case StateClosed:
			stateCode = 0
		case StateHalfOpen:
			stateCode = 1
		case StateOpen:
			stateCode = 2
		}
		cb.monitor.SetCBState(cb.targetName, stateCode)
	}
}

func (cb *CircuitBreaker) Execute(job func() (any, error)) (any, error) {
	cb.mu.Lock()

	if cb.state == StateOpen {
		if time.Since(cb.lastFailureTime) > cb.recoveryTimeout {
			cb.changeState(StateHalfOpen)
		} else {
			cb.mu.Unlock()

			if cb.monitor != nil {
				cb.monitor.IncCBRejected(cb.targetName)
			}

			return nil, ErrCircuitOpen
		}
	}

	cb.mu.Unlock()

	result, err := job()

	cb.mu.Lock()
	defer cb.mu.Unlock()

	if err != nil {
		cb.failureCount++
		cb.lastFailureTime = time.Now()

		if cb.failureCount >= cb.failureThreshold {
			cb.changeState(StateOpen)
		}

		return nil, err
	}

	switch cb.state {
	case StateHalfOpen:
		cb.changeState(StateClosed)
		cb.failureCount = 0
	case StateClosed:
		cb.failureCount = 0
	}

	return result, nil
}
