package response

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"

	"github.com/arixbit/ginblade/internal/errcode"
	customvalidator "github.com/arixbit/ginblade/pkg/validator"
)

func newTestContext() (echo.Context, *httptest.ResponseRecorder) {
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	return echo.New().NewContext(req, w), w
}

func TestSuccessResponse(t *testing.T) {
	res := SuccessResponse(map[string]string{"k": "v"})
	if res.Code != 0 {
		t.Fatalf("Code = %d, want 0", res.Code)
	}
	if res.Message != "success" {
		t.Fatalf("Message = %q, want success", res.Message)
	}
}

func TestErrorResponse(t *testing.T) {
	c, _ := newTestContext()
	c.Set("trace_id", "trace-1")

	res := ErrorResponse(c, errcode.InvalidParams)
	if res.Code != errcode.InvalidParams.Code() {
		t.Fatalf("Code = %d, want %d", res.Code, errcode.InvalidParams.Code())
	}
	if res.Reason != "INVALID_PARAMS" {
		t.Fatalf("Reason = %q, want INVALID_PARAMS", res.Reason)
	}
	if res.Message != "invalid request parameters" {
		t.Fatalf("Message = %q, want invalid request parameters", res.Message)
	}
	if res.Metadata["trace_id"] != "trace-1" {
		t.Fatalf("Metadata = %v, want trace_id trace-1", res.Metadata)
	}
}

func TestErrorResponseWithoutTrace(t *testing.T) {
	c, _ := newTestContext()
	res := ErrorResponse(c, errcode.Unauthorized)
	if res.Metadata != nil {
		t.Fatalf("Metadata = %v, want nil when no trace id", res.Metadata)
	}
}

func TestBuildValidationErrorResponse(t *testing.T) {
	c, _ := newTestContext()
	res := BuildValidationErrorResponse(c, errors.New("bad request"))
	if res.Code != errcode.InvalidParams.Code() {
		t.Fatalf("Code = %d, want %d", res.Code, errcode.InvalidParams.Code())
	}
	if res.Message != "bad request" {
		t.Fatalf("Message = %q, want bad request", res.Message)
	}
}

func TestWriteSuccess(t *testing.T) {
	c, w := newTestContext()
	WriteSuccess(c, map[string]string{"k": "v"})

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	var body Response
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if body.Code != 0 {
		t.Fatalf("Code = %d, want 0", body.Code)
	}
}

func TestWriteErrorKnownCode(t *testing.T) {
	c, w := newTestContext()
	WriteError(c, errcode.DatabaseError)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (business errors use 200 by convention)", w.Code)
	}
	var body Response
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if body.Code != errcode.DatabaseError.Code() {
		t.Fatalf("Code = %d, want %d", body.Code, errcode.DatabaseError.Code())
	}
}

func TestWriteErrorUnknownError(t *testing.T) {
	c, w := newTestContext()
	WriteError(c, errors.New("unexpected"))

	var body Response
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if body.Code != errcode.InternalError.Code() {
		t.Fatalf("Code = %d, want %d (internal)", body.Code, errcode.InternalError.Code())
	}
}

func TestWriteValidationError(t *testing.T) {
	c, w := newTestContext()
	WriteValidationError(c, errors.New("bad"))

	var body Response
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if body.Code != errcode.InvalidParams.Code() {
		t.Fatalf("Code = %d, want %d", body.Code, errcode.InvalidParams.Code())
	}
}

func TestMessageFor(t *testing.T) {
	cases := map[string]string{
		"INVALID_PARAMS":    "invalid request parameters",
		"UNAUTHORIZED":      "unauthorized",
		"PERMISSION_DENIED": "permission denied",
		"TOO_MANY_REQUESTS": "too many requests",
		"REQUEST_TIMEOUT":   "request timeout",
		"DATABASE_ERROR":    "database error",
		"QUEUE_UNAVAILABLE": "queue unavailable",
		"QUEUE_ERROR":       "queue error",
		"UNKNOWN_REASON":    "operation failed",
	}
	for reason, want := range cases {
		if got := messageFor(reason); got != want {
			t.Errorf("messageFor(%q) = %q, want %q", reason, got, want)
		}
	}
}

func TestBuildMetadata(t *testing.T) {
	c, _ := newTestContext()
	if got := buildMetadata(c); got != nil {
		t.Fatalf("buildMetadata without trace = %v, want nil", got)
	}
	c.Set("trace_id", "t-1")
	got := buildMetadata(c)
	if got["trace_id"] != "t-1" {
		t.Fatalf("buildMetadata = %v, want trace_id t-1", got)
	}
}

func TestJSONEnvelopeShape(t *testing.T) {
	c, _ := newTestContext()
	res := ErrorResponse(c, errcode.QueueUnavailable)
	data, err := json.Marshal(res)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	raw := string(data)
	for _, field := range []string{`"code"`, `"msg"`, `"reason"`} {
		if !strings.Contains(raw, field) {
			t.Errorf("envelope missing %s: %s", field, raw)
		}
	}
}

func TestBuildValidationErrorResponseFromValidationErrors(t *testing.T) {
	c, _ := newTestContext()
	err := customvalidator.New().Validate(struct {
		Name string `validate:"required"`
	}{})
	if err == nil {
		t.Fatal("expected a validation error")
	}
	if got := BuildValidationErrorResponse(c, err).Message; got != "name is required" {
		t.Fatalf("Message = %q, want 'name is required'", got)
	}
}

func TestBuildValidationErrorResponseFromHTTPError(t *testing.T) {
	c, _ := newTestContext()

	withInternal := echo.NewHTTPError(http.StatusBadRequest, "outer").SetInternal(errors.New("inner detail"))
	if got := BuildValidationErrorResponse(c, withInternal).Message; got != "inner detail" {
		t.Fatalf("Message = %q, want the internal error 'inner detail'", got)
	}

	messageOnly := echo.NewHTTPError(http.StatusUnsupportedMediaType, "Unsupported Media Type")
	if got := BuildValidationErrorResponse(c, messageOnly).Message; got != "Unsupported Media Type" {
		t.Fatalf("Message = %q, want 'Unsupported Media Type'", got)
	}

	nonString := echo.NewHTTPError(http.StatusBadRequest, map[string]string{"k": "v"})
	if got := BuildValidationErrorResponse(c, nonString).Message; got != nonString.Error() {
		t.Fatalf("Message = %q, want the error string fallback %q", got, nonString.Error())
	}
}
