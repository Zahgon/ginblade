package middleware

import (
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"go.uber.org/zap"

	applog "github.com/arixbit/ginblade/pkg/log"
)

// TraceLogger attaches a trace ID to each request and optionally logs request lifecycle fields.
func TraceLogger(auditEnabled bool, auditExcludes []string) echo.MiddlewareFunc {
	excludes := make(map[string]struct{}, len(auditExcludes))
	for _, path := range auditExcludes {
		if path != "" {
			excludes[path] = struct{}{}
		}
	}

	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			start := time.Now()
			req := c.Request()
			traceID := req.Header.Get("X-Request-ID")
			if traceID == "" {
				traceID = uuid.NewString()
			}
			c.Set("trace_id", traceID)
			c.Response().Header().Set("X-Request-ID", traceID)
			c.SetRequest(req.WithContext(applog.WithTraceID(req.Context(), traceID)))

			// Handle the error here so the recorded status reflects the written response.
			if err := next(c); err != nil {
				c.Error(err)
			}

			if !auditEnabled {
				return nil
			}
			if _, skip := excludes[c.Request().URL.Path]; skip {
				return nil
			}
			applog.FromContext(c.Request().Context()).Info("http request completed",
				zap.String("method", c.Request().Method),
				zap.String("path", c.Request().URL.Path),
				zap.Int("status", c.Response().Status),
				zap.Duration("latency", time.Since(start)),
				zap.String("client_ip", c.RealIP()),
			)
			return nil
		}
	}
}
