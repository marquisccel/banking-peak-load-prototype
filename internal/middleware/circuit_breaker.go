package middleware

import (
	"errors"
	"net/http"
	"time"

	"github.com/ahargunyllib/banking-peak-load-prototype/internal/config"
	"github.com/ahargunyllib/banking-peak-load-prototype/internal/metrics"
	"github.com/labstack/echo/v5"
	"github.com/sony/gobreaker"
)

// CircuitBreaker returns an Echo middleware that wraps all routes in a single
// circuit breaker. The breaker opens after CBMaxFailures consecutive 5xx
// responses and half-opens after CBTimeoutSeconds. When open it immediately
// returns HTTP 503 without calling the handler. When CIRCUIT_BREAKER_ENABLED
// is false the middleware is a transparent no-op.
func CircuitBreaker(cfg *config.Config) echo.MiddlewareFunc {
	if !cfg.CircuitBreakerEnabled {
		return func(next echo.HandlerFunc) echo.HandlerFunc { return next }
	}

	const dependency = "api"
	metrics.CircuitBreakerState.WithLabelValues(dependency).Set(circuitBreakerStateValue(gobreaker.StateClosed))
	metrics.CircuitBreakerOpen.Set(0)

	cb := gobreaker.NewCircuitBreaker(gobreaker.Settings{
		Name:        "api",
		MaxRequests: 1, // allow 1 probe in half-open state
		Interval:    0, // never reset counts while closed
		Timeout:     time.Duration(cfg.CBTimeoutSeconds) * time.Second,
		ReadyToTrip: func(counts gobreaker.Counts) bool {
			return int(counts.ConsecutiveFailures) >= cfg.CBMaxFailures
		},
		OnStateChange: func(_ string, _ gobreaker.State, to gobreaker.State) {
			metrics.CircuitBreakerState.WithLabelValues(dependency).Set(circuitBreakerStateValue(to))
			if to == gobreaker.StateOpen {
				metrics.CircuitBreakerOpen.Set(1)
			} else {
				metrics.CircuitBreakerOpen.Set(0)
			}
		},
	})

	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			_, cbErr := cb.Execute(func() (any, error) {
				handlerErr := next(c)
				if handlerErr != nil {
					return nil, handlerErr
				}
				if resp, err := echo.UnwrapResponse(c.Response()); err == nil && resp.Status >= 500 {
					return nil, echo.NewHTTPError(resp.Status, http.StatusText(resp.Status))
				}
				return nil, nil
			})

			if errors.Is(cbErr, gobreaker.ErrOpenState) || errors.Is(cbErr, gobreaker.ErrTooManyRequests) {
				return c.JSON(http.StatusServiceUnavailable, map[string]string{"error": "circuit open, try again later"})
			}
			return cbErr
		}
	}
}

func circuitBreakerStateValue(state gobreaker.State) float64 {
	switch state {
	case gobreaker.StateOpen:
		return 1
	case gobreaker.StateHalfOpen:
		return 2
	default:
		return 0
	}
}
