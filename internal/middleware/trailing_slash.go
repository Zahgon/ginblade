package middleware

import (
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"
)

// RedirectTrailingSlash reproduces Gin's RedirectTrailingSlash behaviour, which
// Echo has no equivalent for.
//
// Gin answers a request whose path carries a trailing slash with a redirect to
// the canonical path, but only when a route actually exists for the trimmed
// path; otherwise the request falls through to the not-found handler. Echo
// simply does not match, so before this middleware existed a client that
// appended a trailing slash received a hard 404 where it previously received a
// 301 and reached the resource.
//
// routeExists reports whether the trimmed path is a registered route. It is
// supplied by the caller because the route table is only complete once every
// route has been registered.
//
// The redirect is written with http.Redirect, which is what Gin uses, so the
// status line, the Location header, the text/html body and its Content-Type all
// match the original byte for byte. This middleware must be registered before
// any middleware that decorates the response (the trace logger, for example),
// because in Gin the redirect is issued by the router before handlers run and
// therefore carries none of those decorations.
func RedirectTrailingSlash(routeExists func(path string) bool) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			request := c.Request()
			path := request.URL.Path

			if path == "/" || !strings.HasSuffix(path, "/") {
				return next(c)
			}

			trimmed := strings.TrimRight(path, "/")
			if trimmed == "" || routeExists == nil || !routeExists(trimmed) {
				return next(c)
			}

			// Gin uses 301 for GET and 308 for every other method, so that the
			// method and body of a non-GET request are preserved on replay.
			code := http.StatusMovedPermanently
			if request.Method != http.MethodGet {
				code = http.StatusPermanentRedirect
			}

			target := trimmed
			if request.URL.RawQuery != "" {
				target += "?" + request.URL.RawQuery
			}

			http.Redirect(c.Response(), request, target, code)
			return nil
		}
	}
}
