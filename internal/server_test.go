package app

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
	"go.uber.org/zap"

	"github.com/arixbit/ginblade/config"
	"github.com/arixbit/ginblade/internal/bootstrap"
	"github.com/arixbit/ginblade/pkg/database"
	applog "github.com/arixbit/ginblade/pkg/log"
)

func init() {
	applog.SetLogger(zap.NewNop())
}

func TestValidateHTTPRegistry(t *testing.T) {
	if err := validateHTTPRegistry(nil); err != errNilRegistry {
		t.Fatalf("nil registry = %v, want errNilRegistry", err)
	}
	if err := validateHTTPRegistry(&bootstrap.Registry{}); err != errNilConfig {
		t.Fatalf("nil config = %v, want errNilConfig", err)
	}
	if err := validateHTTPRegistry(&bootstrap.Registry{Cfg: &config.Config{}}); err != errMissingDB {
		t.Fatalf("missing db = %v, want errMissingDB", err)
	}
	reg := &bootstrap.Registry{
		Cfg: &config.Config{},
		DB:  database.NewTestManager(nil),
	}
	if err := validateHTTPRegistry(reg); err != errMissingDB {
		t.Fatalf("nil db handle = %v, want errMissingDB", err)
	}
}

func TestValidateWorkerRegistry(t *testing.T) {
	if err := validateWorkerRegistry(nil); err != errNilRegistry {
		t.Fatalf("nil registry = %v, want errNilRegistry", err)
	}
	if err := validateWorkerRegistry(&bootstrap.Registry{}); err != errNilConfig {
		t.Fatalf("nil config = %v, want errNilConfig", err)
	}
	if err := validateWorkerRegistry(&bootstrap.Registry{Cfg: &config.Config{}}); err == nil {
		t.Fatal("expected error for missing redis address")
	}
	reg := &bootstrap.Registry{Cfg: &config.Config{Redis: config.RedisConfig{Addr: "redis:6379"}}}
	if err := validateWorkerRegistry(reg); err != nil {
		t.Fatalf("valid registry: %v", err)
	}
}

func TestNewHTTPHandlersWithNilResources(t *testing.T) {
	reg := &bootstrap.Registry{
		Cfg: &config.Config{},
		DB:  database.NewTestManager(nil),
	}
	handlers := newHTTPHandlers(reg)
	if handlers == nil {
		t.Fatal("expected non-nil handlers")
	}
	if handlers.Example == nil {
		t.Error("expected example handler")
	}
	if handlers.Health == nil {
		t.Error("expected health handler")
	}
	if handlers.Auth != nil {
		t.Error("expected nil auth handler without JWT manager")
	}
}

func TestNewHTTPServer(t *testing.T) {
	cfg := &config.Config{
		Server: config.ServerConfig{
			Port:           ":9999",
			RequestTimeout: 5 * time.Second,
		},
	}
	srv := newHTTPServer(cfg, echo.New())
	if srv == nil {
		t.Fatal("expected non-nil http server")
	}
	if srv.Addr != ":9999" {
		t.Fatalf("Addr = %q, want :9999", srv.Addr)
	}
	if srv.ReadHeaderTimeout != 5*time.Second {
		t.Fatalf("ReadHeaderTimeout = %v, want 5s", srv.ReadHeaderTimeout)
	}
	if srv.Handler == nil {
		t.Fatal("expected non-nil handler")
	}
}

func TestBuildWorkerDeps(t *testing.T) {
	reg := &bootstrap.Registry{
		Cfg:   &config.Config{Redis: config.RedisConfig{Addr: "redis:6379"}},
		DB:    database.NewTestManager(nil),
		Cache: nil,
		Queue: nil,
	}
	deps := buildWorkerDeps(reg)
	if deps == nil {
		t.Fatal("expected non-nil deps")
	}
	if deps.DB != nil {
		t.Error("expected nil DB when manager has no handle")
	}
}

func TestServerRunNilHTTP(t *testing.T) {
	s := &Server{}
	if err := s.Run(); err != errNilHTTPServer {
		t.Fatalf("Run = %v, want errNilHTTPServer", err)
	}
}

func TestServerShutdownNilHTTP(t *testing.T) {
	s := &Server{}
	if err := s.Shutdown(t.Context()); err != errNilHTTPServer {
		t.Fatalf("Shutdown = %v, want errNilHTTPServer", err)
	}
}

func TestServerCloseNilHTTP(t *testing.T) {
	s := &Server{}
	if err := s.Close(); err != errNilHTTPServer {
		t.Fatalf("Close = %v, want errNilHTTPServer", err)
	}
}

func TestNewServerNilRegistry(t *testing.T) {
	if _, err := NewServer(nil); err != errNilRegistry {
		t.Fatalf("NewServer(nil) = %v, want errNilRegistry", err)
	}
}

func TestWorkerRunNilServer(t *testing.T) {
	w := &Worker{}
	if err := w.Run(t.Context()); err != errNilWorker {
		t.Fatalf("Run = %v, want errNilWorker", err)
	}
}

func TestNewWorkerNilRegistry(t *testing.T) {
	if _, err := NewWorker(nil); err != errNilRegistry {
		t.Fatalf("NewWorker(nil) = %v, want errNilRegistry", err)
	}
}

func TestHTTPServerUsesConfigPort(t *testing.T) {
	cfg := &config.Config{Server: config.ServerConfig{Port: ":1234"}}
	srv := newHTTPServer(cfg, echo.New())
	if srv.Addr != ":1234" {
		t.Fatalf("Addr = %q, want :1234", srv.Addr)
	}
	if srv.Handler == nil || srv.Handler == http.NotFoundHandler() {
		t.Fatal("expected engine handler")
	}
}

func TestParseTrustedProxy(t *testing.T) {
	for _, proxy := range []string{"10.0.0.0/8", "192.168.1.1", "::1", "2001:db8::/32"} {
		if _, err := parseTrustedProxy(proxy); err != nil {
			t.Errorf("parseTrustedProxy(%q) = %v, want no error", proxy, err)
		}
	}
	for _, proxy := range []string{"not-an-ip", "10.0.0.0/99", ""} {
		if _, err := parseTrustedProxy(proxy); err == nil {
			t.Errorf("parseTrustedProxy(%q) = nil, want error", proxy)
		}
	}

	ipNet, err := parseTrustedProxy("192.168.1.1")
	if err != nil {
		t.Fatalf("parseTrustedProxy: %v", err)
	}
	if got := ipNet.String(); got != "192.168.1.1/32" {
		t.Errorf("bare IPv4 = %q, want 192.168.1.1/32", got)
	}
	if ipNet, err = parseTrustedProxy("::1"); err != nil {
		t.Fatalf("parseTrustedProxy: %v", err)
	}
	if got := ipNet.String(); got != "::1/128" {
		t.Errorf("bare IPv6 = %q, want ::1/128", got)
	}
}

func TestNewIPExtractorInvalidProxy(t *testing.T) {
	if _, err := newIPExtractor([]string{"10.0.0.0/8", "bogus"}); err == nil {
		t.Fatal("expected error for invalid trusted proxy")
	}
}

func extractIP(t *testing.T, proxies []string, remoteAddr string, headers map[string]string) string {
	t.Helper()
	extract, err := newIPExtractor(proxies)
	if err != nil {
		t.Fatalf("newIPExtractor: %v", err)
	}
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = remoteAddr
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	return extract(req)
}

func TestIPExtractorWithoutTrustedProxies(t *testing.T) {
	got := extractIP(t, nil, "203.0.113.9:1234", map[string]string{
		"X-Forwarded-For": "1.2.3.4",
		"X-Real-IP":       "5.6.7.8",
	})
	if got != "203.0.113.9" {
		t.Fatalf("client ip = %q, want the peer address 203.0.113.9", got)
	}
}

func TestIPExtractorForwardingHeaders(t *testing.T) {
	proxies := []string{"203.0.113.0/24"}
	cases := []struct {
		name       string
		remoteAddr string
		headers    map[string]string
		want       string
	}{
		{
			name:       "x-forwarded-for",
			remoteAddr: "203.0.113.9:1234",
			headers:    map[string]string{"X-Forwarded-For": "1.2.3.4"},
			want:       "1.2.3.4",
		},
		{
			name:       "nearest untrusted hop wins",
			remoteAddr: "203.0.113.9:1234",
			headers:    map[string]string{"X-Forwarded-For": "1.2.3.4, 9.9.9.9, 203.0.113.7"},
			want:       "9.9.9.9",
		},
		{
			name:       "all hops trusted falls back to the furthest",
			remoteAddr: "203.0.113.9:1234",
			headers:    map[string]string{"X-Forwarded-For": "203.0.113.1, 203.0.113.7"},
			want:       "203.0.113.1",
		},
		{
			name:       "x-real-ip when no x-forwarded-for",
			remoteAddr: "203.0.113.9:1234",
			headers:    map[string]string{"X-Real-IP": "1.2.3.4"},
			want:       "1.2.3.4",
		},
		{
			name:       "x-forwarded-for takes precedence over x-real-ip",
			remoteAddr: "203.0.113.9:1234",
			headers:    map[string]string{"X-Forwarded-For": "1.2.3.4", "X-Real-IP": "5.6.7.8"},
			want:       "1.2.3.4",
		},
		{
			name:       "malformed x-forwarded-for falls through to x-real-ip",
			remoteAddr: "203.0.113.9:1234",
			headers:    map[string]string{"X-Forwarded-For": "not-an-ip", "X-Real-IP": "5.6.7.8"},
			want:       "5.6.7.8",
		},
		{
			name:       "untrusted peer ignores forwarding headers",
			remoteAddr: "198.51.100.4:1234",
			headers:    map[string]string{"X-Forwarded-For": "1.2.3.4", "X-Real-IP": "5.6.7.8"},
			want:       "198.51.100.4",
		},
		{
			name:       "private peer is not trusted unless configured",
			remoteAddr: "10.1.2.3:1234",
			headers:    map[string]string{"X-Forwarded-For": "1.2.3.4"},
			want:       "10.1.2.3",
		},
		{
			name:       "no forwarding headers falls back to the peer",
			remoteAddr: "203.0.113.9:1234",
			want:       "203.0.113.9",
		},
		{
			name:       "unparsable peer address",
			remoteAddr: "garbage",
			headers:    map[string]string{"X-Forwarded-For": "1.2.3.4"},
			want:       "",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := extractIP(t, proxies, tc.remoteAddr, tc.headers); got != tc.want {
				t.Fatalf("client ip = %q, want %q", got, tc.want)
			}
		})
	}
}
