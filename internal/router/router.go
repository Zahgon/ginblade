package router

import (
	"github.com/labstack/echo/v4"

	"github.com/arixbit/ginblade/internal/handler"
)

// Dependencies collects handlers and middleware needed during route registration.
type Dependencies struct {
	Auth         *handler.AuthHandler
	AuthRequired echo.MiddlewareFunc
	Example      *handler.ExampleHandler
}

// RegisterRoutes registers API routes under the given router group.
func RegisterRoutes(r *echo.Group, deps Dependencies) error {
	registerAuthRoutes(r, deps)
	registerExampleRoutes(r, deps)
	return nil
}

func registerAuthRoutes(r *echo.Group, deps Dependencies) {
	if deps.Auth == nil {
		return
	}

	authRoutes := r.Group("/auth")
	authRoutes.POST("/token", deps.Auth.CreateToken)
	if deps.AuthRequired != nil {
		authRoutes.GET("/me", deps.Auth.Me, deps.AuthRequired)
	}
}

func registerExampleRoutes(r *echo.Group, deps Dependencies) {
	if deps.Example == nil {
		return
	}

	examples := r.Group("/examples")
	examples.GET("", deps.Example.List)
	examples.POST("", deps.Example.Create)
	examples.POST("/tasks", deps.Example.EnqueueTask)
}
