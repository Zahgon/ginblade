package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/labstack/echo/v4"

	"github.com/arixbit/ginblade/internal/errcode"
	"github.com/arixbit/ginblade/pkg/auth"
	"github.com/arixbit/ginblade/pkg/response"
)

func TestBearerAuthAcceptsValidToken(t *testing.T) {
	manager, err := auth.NewJWTManager(auth.JWTConfig{
		Secret: "test-secret",
		Issuer: "ginblade-test",
		TTL:    time.Hour,
	})
	if err != nil {
		t.Fatalf("NewJWTManager: %v", err)
	}
	token, err := manager.GenerateToken("subject-1")
	if err != nil {
		t.Fatalf("GenerateToken: %v", err)
	}

	router := echo.New()
	router.GET("/me", func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]any{"subject": AuthSubject(c)})
	}, BearerAuth(manager))

	req := httptest.NewRequest(http.MethodGet, "/me", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var body struct {
		Subject string `json:"subject"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if body.Subject != "subject-1" {
		t.Fatalf("expected subject-1, got %q", body.Subject)
	}
}

func TestBearerAuthRejectsMissingToken(t *testing.T) {
	manager, err := auth.NewJWTManager(auth.JWTConfig{Secret: "test-secret"})
	if err != nil {
		t.Fatalf("NewJWTManager: %v", err)
	}

	router := echo.New()
	router.GET("/me", func(c echo.Context) error {
		return c.NoContent(http.StatusOK)
	}, BearerAuth(manager))

	req := httptest.NewRequest(http.MethodGet, "/me", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	var body response.Response
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if body.Code != errcode.Unauthorized.Code() {
		t.Fatalf("expected unauthorized code, got %d", body.Code)
	}
}
