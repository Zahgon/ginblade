package app

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"

	"github.com/arixbit/ginblade/config"
	"github.com/arixbit/ginblade/internal/bootstrap"
	"github.com/arixbit/ginblade/internal/handler"
	"github.com/arixbit/ginblade/internal/middleware"
	"github.com/arixbit/ginblade/internal/repository"
	"github.com/arixbit/ginblade/internal/router"
	"github.com/arixbit/ginblade/internal/service"
	customvalidator "github.com/arixbit/ginblade/pkg/validator"
)

var (
	errNilRegistry   = errors.New("app: nil registry")
	errNilConfig     = errors.New("app: nil config")
	errMissingDB     = errors.New("app: missing database")
	errNilHTTPServer = errors.New("app: nil http server")
	errNilWorker     = errors.New("app: nil worker")
)

// HTTPHandlers groups pre-constructed handlers used by the HTTP server.
type HTTPHandlers struct {
	Auth    *handler.AuthHandler
	Health  *handler.HealthHandler
	Example *handler.ExampleHandler
}

// Server owns the HTTP transport created from application dependencies.
type Server struct {
	Engine      *echo.Echo
	HTTP        *http.Server
	Handlers    *HTTPHandlers
	rateLimiter *middleware.IPRateLimiter
}

// NewServer wires HTTP handlers, middleware, and the underlying http.Server.
func NewServer(reg *bootstrap.Registry) (*Server, error) {
	if err := validateHTTPRegistry(reg); err != nil {
		return nil, err
	}

	var rl *middleware.IPRateLimiter
	if rpm := reg.Cfg.RateLimit.RequestsPerMinute; rpm > 0 {
		rl = middleware.NewIPRateLimiterPerMinute(rpm)
	}

	handlers := newHTTPHandlers(reg)
	engine, err := newEngine(reg, handlers, rl)
	if err != nil {
		return nil, err
	}

	return &Server{
		Engine:      engine,
		HTTP:        newHTTPServer(reg.Cfg, engine),
		Handlers:    handlers,
		rateLimiter: rl,
	}, nil
}

// Run starts serving HTTP requests until Shutdown is called.
func (s *Server) Run() error {
	if s == nil || s.HTTP == nil {
		return errNilHTTPServer
	}
	if err := s.HTTP.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("listen and serve http server: %w", err)
	}
	return nil
}

// Shutdown gracefully stops the HTTP server and releases owned resources.
func (s *Server) Shutdown(ctx context.Context) error {
	if s == nil || s.HTTP == nil {
		return errNilHTTPServer
	}
	if s.rateLimiter != nil {
		s.rateLimiter.Stop()
	}
	return s.HTTP.Shutdown(ctx)
}

// Close immediately closes the HTTP server and releases owned resources.
func (s *Server) Close() error {
	if s == nil || s.HTTP == nil {
		return errNilHTTPServer
	}
	if s.rateLimiter != nil {
		s.rateLimiter.Stop()
	}
	if err := s.HTTP.Close(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

func validateHTTPRegistry(reg *bootstrap.Registry) error {
	switch {
	case reg == nil:
		return errNilRegistry
	case reg.Cfg == nil:
		return errNilConfig
	case reg.DB == nil || reg.DB.DB() == nil:
		return errMissingDB
	default:
		return nil
	}
}

func newHTTPHandlers(reg *bootstrap.Registry) *HTTPHandlers {
	db := reg.DB.DB()
	exampleRepository := repository.NewExampleRepository(db)
	exampleService := service.NewExampleService(exampleRepository, reg.Queue)

	return &HTTPHandlers{
		Auth:    handler.NewAuthHandler(reg.Auth),
		Health:  handler.NewHealthHandler(reg.DB, reg.Cache),
		Example: handler.NewExampleHandler(exampleService),
	}
}

func newEngine(reg *bootstrap.Registry, handlers *HTTPHandlers, rl *middleware.IPRateLimiter) (*echo.Echo, error) {
	engine := echo.New()
	engine.Debug = strings.EqualFold(strings.TrimSpace(reg.Cfg.Server.Mode), "debug")
	engine.Validator = customvalidator.New()

	extractor, err := newIPExtractor(reg.Cfg.Server.TrustedProxies)
	if err != nil {
		return nil, fmt.Errorf("set trusted proxies: %w", err)
	}
	engine.IPExtractor = extractor

	// Registered before every other middleware so the redirect carries none of
	// their decorations, matching Gin, where the router issues it before handlers
	// run. The route set is populated at the end of this function; the closure
	// reads it per request, by which time it is complete.
	registeredPaths := make(map[string]struct{})
	engine.Use(middleware.RedirectTrailingSlash(func(path string) bool {
		_, ok := registeredPaths[path]
		return ok
	}))

	engine.Use(middleware.TraceLogger(reg.Cfg.Log.AuditEnabled, reg.Cfg.Log.AuditExcludes))
	engine.Use(middleware.Recovery())
	engine.Use(middleware.Timeout(reg.Cfg.Server.RequestTimeout))
	engine.Use(middleware.CORS(reg.Cfg.Cors.AllowOrigins))
	if rl != nil {
		engine.Use(rl.Middleware())
	}

	engine.GET("/health", handlers.Health.Health)
	api := engine.Group("/api/v1")

	var authRequired echo.MiddlewareFunc
	if reg.Auth != nil {
		authRequired = middleware.BearerAuth(reg.Auth)
	}
	if err := router.RegisterRoutes(api, router.Dependencies{
		Auth:         handlers.Auth,
		AuthRequired: authRequired,
		Example:      handlers.Example,
	}); err != nil {
		return nil, err
	}

	for _, route := range engine.Routes() {
		registeredPaths[route.Path] = struct{}{}
	}

	engine.HTTPErrorHandler = ginCompatibleErrorHandler(engine)

	return engine, nil
}

// ginCompatibleErrorHandler renders routing failures the way Gin did.
//
// Gin does not distinguish an unknown path from an unsupported method: both are
// answered with 404 and the plain-text body "404 page not found", and no Allow
// header is sent. Echo's default handler answers 405 with an Allow header for the
// unsupported-method case and a JSON object for both, which changes the status
// code clients branch on. Everything that is not a routing failure is delegated
// to Echo's default handler unchanged.
func ginCompatibleErrorHandler(engine *echo.Echo) echo.HTTPErrorHandler {
	fallback := engine.DefaultHTTPErrorHandler
	return func(err error, c echo.Context) {
		if c.Response().Committed {
			return
		}

		var httpErr *echo.HTTPError
		if errors.As(err, &httpErr) &&
			(httpErr.Code == http.StatusNotFound || httpErr.Code == http.StatusMethodNotAllowed) {
			c.Response().Header().Del(echo.HeaderAllow)
			if blobErr := c.Blob(
				http.StatusNotFound,
				"text/plain",
				[]byte("404 page not found"),
			); blobErr != nil {
				c.Logger().Error(blobErr)
			}
			return
		}

		fallback(err, c)
	}
}

// newIPExtractor builds the client IP resolver from the configured trusted
// proxies. With no trusted proxy the peer address is used. Otherwise, when the
// peer is a trusted proxy, the client address is taken from X-Forwarded-For
// (nearest untrusted hop) and then X-Real-IP, falling back to the peer address.
func newIPExtractor(trustedProxies []string) (echo.IPExtractor, error) {
	if len(trustedProxies) == 0 {
		return echo.ExtractIPDirect(), nil
	}

	trustedCIDRs := make([]*net.IPNet, 0, len(trustedProxies))
	for _, proxy := range trustedProxies {
		ipNet, err := parseTrustedProxy(proxy)
		if err != nil {
			return nil, err
		}
		trustedCIDRs = append(trustedCIDRs, ipNet)
	}

	trusted := func(ip net.IP) bool {
		for _, cidr := range trustedCIDRs {
			if cidr.Contains(ip) {
				return true
			}
		}
		return false
	}

	remoteAddr := echo.ExtractIPDirect()
	return func(req *http.Request) string {
		remote := remoteAddr(req)
		remoteIP := net.ParseIP(remote)
		if remoteIP == nil {
			return ""
		}
		if !trusted(remoteIP) {
			return remoteIP.String()
		}
		for _, header := range []string{echo.HeaderXForwardedFor, echo.HeaderXRealIP} {
			if ip, ok := clientIPFromHeader(req.Header.Get(header), trusted); ok {
				return ip
			}
		}
		return remoteIP.String()
	}, nil
}

// clientIPFromHeader walks a forwarding header from the closest hop outward and
// returns the first address that is not itself a trusted proxy.
func clientIPFromHeader(header string, trusted func(net.IP) bool) (string, bool) {
	if header == "" {
		return "", false
	}

	items := strings.Split(header, ",")
	for i := len(items) - 1; i >= 0; i-- {
		ipStr := strings.TrimSpace(items[i])
		ip := net.ParseIP(ipStr)
		if ip == nil {
			break
		}
		if i == 0 || !trusted(ip) {
			return ipStr, true
		}
	}
	return "", false
}

func parseTrustedProxy(proxy string) (*net.IPNet, error) {
	if !strings.Contains(proxy, "/") {
		ip := net.ParseIP(proxy)
		if ip == nil {
			return nil, &net.ParseError{Type: "IP address", Text: proxy}
		}
		if ip.To4() != nil {
			proxy += "/32"
		} else {
			proxy += "/128"
		}
	}

	_, ipNet, err := net.ParseCIDR(proxy)
	if err != nil {
		return nil, err
	}
	return ipNet, nil
}

func newHTTPServer(cfg *config.Config, engine *echo.Echo) *http.Server {
	return &http.Server{
		Addr:              cfg.Server.Port,
		Handler:           engine,
		ReadHeaderTimeout: cfg.Server.RequestTimeout,
	}
}
