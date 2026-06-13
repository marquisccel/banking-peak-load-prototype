package middleware

import (
	"time"

	"github.com/labstack/echo/v5"
	"github.com/marquisccel/banking-peak-load-prototype/internal/logger"
)

// RequestLogger returns an Echo middleware that implements the wide event /
// canonical log line pattern: one structured JSON log per request, emitted at
// completion, with all relevant context accumulated during the request.
func RequestLogger() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			start := time.Now()

			// Attach a fresh wide event to the request context so handlers and
			// services can add business fields via logger.Set.
			event := logger.NewEvent()
			event["method"] = c.Request().Method
			event["path"] = c.Request().URL.Path

			ctx := logger.WithEvent(c.Request().Context(), event)
			c.SetRequest(c.Request().WithContext(ctx))

			handlerErr := next(c)

			if resp, err := echo.UnwrapResponse(c.Response()); err == nil {
				event["status"] = resp.Status
			}
			event["request_id"] = c.Response().Header().Get(echo.HeaderXRequestID)
			event["duration_ms"] = time.Since(start).Milliseconds()
			if handlerErr != nil {
				event["error"] = handlerErr.Error()
			}

			logger.Emit(ctx, event)
			return handlerErr
		}
	}
}
