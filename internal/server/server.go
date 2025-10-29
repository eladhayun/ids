package server

import (
	"time"

	"ids/internal/cache"
	"ids/internal/config"
	"ids/internal/handlers"

	"github.com/jmoiron/sqlx"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/rs/zerolog"
	echoSwagger "github.com/swaggo/echo-swagger"
)

// Server represents the application server
type Server struct {
	echo   *echo.Echo
	db     *sqlx.DB
	config *config.Config
	logger zerolog.Logger
	cache  *cache.Cache
}

// New creates a new server instance
func New(cfg *config.Config, db *sqlx.DB, logger zerolog.Logger) *Server {
	return &Server{
		config: cfg,
		db:     db,
		logger: logger,
		cache:  cache.New(),
	}
}

// zerologMiddleware creates a zerolog-based logging middleware for Echo
func (s *Server) zerologMiddleware() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			start := time.Now()

			err := next(c)

			req := c.Request()
			res := c.Response()

			s.logger.Info().
				Str("method", req.Method).
				Str("uri", req.RequestURI).
				Str("remote_ip", c.RealIP()).
				Int("status", res.Status).
				Int64("latency_ms", time.Since(start).Milliseconds()).
				Str("user_agent", req.UserAgent()).
				Msg("HTTP request")

			return err
		}
	}
}

// Initialize sets up the Echo framework with middleware and routes
func (s *Server) Initialize() {
	s.echo = echo.New()

	// Middleware
	s.echo.Use(s.zerologMiddleware())
	s.echo.Use(middleware.Recover())
	s.echo.Use(middleware.CORS())

	// Hide Echo banner
	s.echo.HideBanner = true

	// Setup routes
	s.setupRoutes()
}

// setupRoutes configures all the application routes
func (s *Server) setupRoutes() {
	// API group with /api prefix
	api := s.echo.Group("/api")

	// Swagger documentation
	s.echo.GET("/swagger/*", echoSwagger.WrapHandler)

	// Health endpoints (keep at root level for monitoring)
	s.echo.GET("/healthz", handlers.HealthHandler(s.config.Version))
	s.echo.GET("/healthz/db", handlers.DBHealthHandler(s.db))

	// API endpoints under /api prefix
	api.GET("/", handlers.RootHandler(s.config.Version))
	api.GET("/products", handlers.ProductsHandler(s.db))
	api.POST("/chat", handlers.ChatHandler(s.db, s.config, s.cache))

	// Serve static files (this should be last to avoid conflicts)
	s.echo.Static("/", "static")
}

// Start starts the HTTP server
func (s *Server) Start() error {
	s.logger.Info().Str("port", s.config.Port).Msg("Server starting")
	return s.echo.Start(":" + s.config.Port)
}
