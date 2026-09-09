package handler

import (
	"encoding/json"

	"github.com/labstack/echo/v4"
)

// queryBinder binds URL query parameters using the `query` struct tag.
var queryBinder = new(echo.DefaultBinder)

// bindJSON decodes the request body as JSON and validates the result. The body
// is always read as JSON regardless of the request Content-Type, and an empty
// body is a decode error.
func bindJSON(c echo.Context, req any) error {
	if err := json.NewDecoder(c.Request().Body).Decode(req); err != nil {
		return err
	}
	return c.Validate(req)
}

// bindQuery binds URL query parameters onto req and validates the result.
func bindQuery(c echo.Context, req any) error {
	if err := queryBinder.BindQueryParams(c, req); err != nil {
		return err
	}
	return c.Validate(req)
}
