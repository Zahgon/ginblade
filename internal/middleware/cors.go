package middleware

import (
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"
)

// CORS returns a simple allow-list based CORS middleware.
func CORS(allowOrigins []string) echo.MiddlewareFunc {
	allowed := make(map[string]struct{}, len(allowOrigins))
	for _, origin := range allowOrigins {
		origin = strings.TrimSpace(origin)
		if origin != "" {
			allowed[origin] = struct{}{}
		}
	}

	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			req := c.Request()
			header := c.Response().Header()

			origin := strings.TrimSpace(req.Header.Get("Origin"))
			if origin != "" && containsOrigin(allowed, origin) {
				header.Set("Access-Control-Allow-Origin", origin)
				header.Set("Vary", "Origin")
				header.Set("Access-Control-Allow-Methods", "GET,POST,PUT,PATCH,DELETE,OPTIONS")
				header.Set("Access-Control-Allow-Headers", "Origin,Content-Type,Accept,Authorization,X-Request-ID")
				header.Set("Access-Control-Allow-Credentials", "true")
			}

			if req.Method == http.MethodOptions {
				return c.NoContent(http.StatusNoContent)
			}
			return next(c)
		}
	}
}

func containsOrigin(allowed map[string]struct{}, origin string) bool {
	_, ok := allowed[origin]
	return ok
}
