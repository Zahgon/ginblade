package response

import (
	"errors"
	"net/http"

	"github.com/go-playground/validator/v10"
	"github.com/labstack/echo/v4"

	"github.com/arixbit/ginblade/internal/errcode"
	customvalidator "github.com/arixbit/ginblade/pkg/validator"
)

// Response is the standard API response structure.
type Response struct {
	Code     int            `json:"code"`
	Message  string         `json:"msg"`
	Reason   string         `json:"reason,omitempty"`
	Data     any            `json:"data,omitempty"`
	Metadata map[string]any `json:"metadata,omitempty"`
}

// SuccessResponse creates a success response with code 0.
func SuccessResponse(data any) Response {
	return Response{Code: 0, Message: "success", Data: data}
}

// ErrorResponse creates an error response.
func ErrorResponse(c echo.Context, errorCode errcode.Error) Response {
	reason := errorCode.Reason()
	return Response{
		Code:     errorCode.Code(),
		Reason:   reason,
		Message:  messageFor(reason),
		Metadata: buildMetadata(c),
	}
}

// BuildValidationErrorResponse creates a validation error response.
func BuildValidationErrorResponse(c echo.Context, err error) Response {
	return Response{
		Code:     errcode.InvalidParams.Code(),
		Reason:   errcode.InvalidParams.Reason(),
		Message:  validationMessage(err),
		Metadata: buildMetadata(c),
	}
}

// WriteSuccess writes a success response with HTTP 200.
func WriteSuccess(c echo.Context, data any) error {
	return c.JSON(http.StatusOK, SuccessResponse(data))
}

// WriteError translates an errcode.Error into the API error envelope.
func WriteError(c echo.Context, err error) error {
	var ec errcode.Error
	if errors.As(err, &ec) {
		return c.JSON(http.StatusOK, ErrorResponse(c, ec))
	}
	return c.JSON(http.StatusOK, ErrorResponse(c, errcode.InternalError))
}

// WriteValidationError writes a validation error response.
func WriteValidationError(c echo.Context, err error) error {
	return c.JSON(http.StatusOK, BuildValidationErrorResponse(c, err))
}

// validationMessage extracts a client-facing message from a bind or validate error.
func validationMessage(err error) string {
	var errs validator.ValidationErrors
	if errors.As(err, &errs) {
		return customvalidator.HandleValidatorError(errs)
	}

	var httpErr *echo.HTTPError
	if errors.As(err, &httpErr) {
		if httpErr.Internal != nil {
			return httpErr.Internal.Error()
		}
		if msg, ok := httpErr.Message.(string); ok {
			return msg
		}
	}
	return err.Error()
}

func messageFor(reason string) string {
	switch reason {
	case "INVALID_PARAMS":
		return "invalid request parameters"
	case "UNAUTHORIZED":
		return "unauthorized"
	case "PERMISSION_DENIED":
		return "permission denied"
	case "TOO_MANY_REQUESTS":
		return "too many requests"
	case "REQUEST_TIMEOUT":
		return "request timeout"
	case "DATABASE_ERROR":
		return "database error"
	case "QUEUE_UNAVAILABLE":
		return "queue unavailable"
	case "QUEUE_ERROR":
		return "queue error"
	default:
		return "operation failed"
	}
}

func buildMetadata(c echo.Context) map[string]any {
	if traceID, ok := c.Get("trace_id").(string); ok && traceID != "" {
		return map[string]any{"trace_id": traceID}
	}
	return nil
}
