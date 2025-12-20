package opensky

import (
	"errors"
	"sync"
	"time"
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
}

func NewCircuitBreaker(threshold int, recoveryTimeout time.Duration) *CircuitBreaker {
	return &CircuitBreaker{
		state:            StateClosed,
		failureThreshold: threshold,
		recoveryTimeout:  recoveryTimeout,
	}
}

func (cb *CircuitBreaker) Execute(job func() (any, error)) (any, error) {
	cb.mu.Lock()

	if cb.state == StateOpen {
		if time.Since(cb.lastFailureTime) > cb.recoveryTimeout {
			cb.state = StateHalfOpen
		} else {
			cb.mu.Unlock()
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
			cb.state = StateOpen
		}

		return nil, err
	}

	switch cb.state {
	case StateHalfOpen:
		cb.state = StateClosed
		cb.failureCount = 0
	case StateClosed:
		cb.failureCount = 0
	}

	return result, nil
}
