package grpc_client

import (
	"context"

	"github.com/DanieleT25/FlightData-Manager/microservices/data-collector/internal/adapters/observability"
	"github.com/sony/gobreaker"
	"google.golang.org/grpc"
)

func circuitBreakerClientInterceptor(cb *gobreaker.CircuitBreaker, monitor *observability.Monitor) grpc.UnaryClientInterceptor {
	return func(
		ctx context.Context,
		method string,
		req, reply any,
		cc *grpc.ClientConn,
		invoker grpc.UnaryInvoker,
		opts ...grpc.CallOption,
	) error {
		_, err := cb.Execute(func() (any, error) {
			err := invoker(ctx, method, req, reply, cc, opts...)
			if err != nil {
				return nil, err
			}
			return nil, nil
		})

		if err != nil {
			if err == gobreaker.ErrOpenState || err == gobreaker.ErrTooManyRequests {
				monitor.IncCBRejected("user_manager")
			}
		}

		return err
	}
}
