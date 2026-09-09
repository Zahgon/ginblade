package middleware

import (
	"runtime/debug"

	"github.com/labstack/echo/v4"
	"go.uber.org/zap"

	"github.com/arixbit/ginblade/internal/errcode"
	applog "github.com/arixbit/ginblade/pkg/log"
	"github.com/arixbit/ginblade/pkg/response"
)

// Recovery catches panics and returns the standard API error envelope.
func Recovery() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) (err error) {
			defer func() {
				if recovered := recover(); recovered != nil {
					req := c.Request()
					applog.FromContext(req.Context()).Error("panic recovered",
						zap.String("method", req.Method),
						zap.String("path", req.URL.Path),
						zap.Any("error", recovered),
						zap.ByteString("stacktrace", debug.Stack()),
					)
					err = c.JSON(200, response.ErrorResponse(c, errcode.InternalError))
				}
			}()
			return next(c)
		}
	}
}
