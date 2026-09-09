package middleware

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/arixbit/ginblade/internal/errcode"
	"github.com/arixbit/ginblade/pkg/auth"
	"github.com/arixbit/ginblade/pkg/response"
)

const authSubjectKey = "auth_subject"

// AuthSubject returns the authenticated subject stored by BearerAuth.
func AuthSubject(c echo.Context) string {
	subject, _ := c.Get(authSubjectKey).(string)
	return subject
}

// BearerAuth validates Authorization Bearer JWT tokens and stores the subject.
func BearerAuth(manager *auth.JWTManager) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			if manager == nil {
				return c.JSON(http.StatusOK, response.ErrorResponse(c, errcode.Unauthorized))
			}

			claims, err := manager.ParseToken(c.Request().Header.Get("Authorization"))
			if err != nil {
				return c.JSON(http.StatusOK, response.ErrorResponse(c, errcode.Unauthorized))
			}

			c.Set(authSubjectKey, claims.Subject)
			return next(c)
		}
	}
}
