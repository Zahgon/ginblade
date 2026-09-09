package handler

import (
	"github.com/labstack/echo/v4"

	"github.com/arixbit/ginblade/internal/service"
	"github.com/arixbit/ginblade/pkg/response"
)

// ExampleHandler handles HTTP requests for examples.
type ExampleHandler struct {
	svc *service.ExampleService
}

// NewExampleHandler creates an ExampleHandler.
func NewExampleHandler(svc *service.ExampleService) *ExampleHandler {
	return &ExampleHandler{svc: svc}
}

// Create handles POST /api/v1/examples.
func (h *ExampleHandler) Create(c echo.Context) error {
	var req service.CreateExampleReq
	if err := bindJSON(c, &req); err != nil {
		return response.WriteValidationError(c, err)
	}

	example, err := h.svc.Create(c.Request().Context(), &req)
	if err != nil {
		return response.WriteError(c, err)
	}

	return response.WriteSuccess(c, example)
}

// List handles GET /api/v1/examples.
func (h *ExampleHandler) List(c echo.Context) error {
	var req service.ListExamplesReq
	if err := bindQuery(c, &req); err != nil {
		return response.WriteValidationError(c, err)
	}

	res, err := h.svc.List(c.Request().Context(), &req)
	if err != nil {
		return response.WriteError(c, err)
	}

	return response.WriteSuccess(c, res)
}

// EnqueueTask handles POST /api/v1/examples/tasks.
func (h *ExampleHandler) EnqueueTask(c echo.Context) error {
	var req service.EnqueueExampleTaskReq
	if err := bindJSON(c, &req); err != nil {
		return response.WriteValidationError(c, err)
	}

	res, err := h.svc.EnqueueTask(c.Request().Context(), &req)
	if err != nil {
		return response.WriteError(c, err)
	}

	return response.WriteSuccess(c, res)
}
