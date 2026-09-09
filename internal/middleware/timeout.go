package middleware

import (
	"context"
	"time"

	"github.com/labstack/echo/v4"
)

// Timeout attaches a deadline to request contexts.
func Timeout(timeout time.Duration) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			if timeout <= 0 {
				return next(c)
			}
			ctx, cancel := context.WithTimeout(c.Request().Context(), timeout)
			defer cancel()
			c.SetRequest(c.Request().WithContext(ctx))
			return next(c)
		}
	}
}
