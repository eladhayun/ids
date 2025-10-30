package server

import (
	"time"

	"ids/internal/cache"
	"ids/internal/config"
	"ids/internal/embeddings"
	"ids/internal/handlers"

	"github.com/jmoiron/sqlx"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/rs/zerolog"
	echoSwagger "github.com/swaggo/echo-swagger"
)

// Server represents the application server
type Server struct {
	echo             *echo.Echo
	db               *sqlx.DB
	config           *config.Config
	logger           zerolog.Logger
	cache            *cache.Cache
	embeddingService *embeddings.EmbeddingService
}

// New creates a new server instance
func New(cfg *config.Config, db *sqlx.DB, logger zerolog.Logger) *Server {
	// Initialize embedding service if OpenAI API key is available
	// Note: This uses read-only access for the main application
	var embeddingService *embeddings.EmbeddingService
	if cfg.OpenAIKey != "" && db != nil {
		var err error
		embeddingService, err = embeddings.NewEmbeddingService(cfg, db)
		if err != nil {
			logger.Warn().Err(err).Msg("Failed to initialize embedding service, falling back to regular chat")
		} else {
			logger.Info().Msg("Embedding service initialized successfully (read-only mode)")
		}
	}

	return &Server{
		config:           cfg,
		db:               db,
		logger:           logger,
		cache:            cache.New(),
		embeddingService: embeddingService,
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

	// Hide Echo banner
	s.echo.HideBanner = true

	// Setup routes
	s.setupRoutes()
}

// setupRoutes configures all the application routes
func (s *Server) setupRoutes() {
	// API group with /api prefix and permissive CORS
	api := s.echo.Group("/api")

	// Configure permissive CORS for all API endpoints
	api.Use(middleware.CORSWithConfig(middleware.CORSConfig{
		AllowOrigins:     []string{"*"}, // Allow all origins
		AllowMethods:     []string{echo.GET, echo.POST, echo.PUT, echo.DELETE, echo.PATCH, echo.OPTIONS, echo.HEAD},
		AllowHeaders:     []string{echo.HeaderOrigin, echo.HeaderContentType, echo.HeaderAccept, echo.HeaderAuthorization, echo.HeaderXRequestedWith},
		ExposeHeaders:    []string{echo.HeaderContentLength, echo.HeaderContentType, echo.HeaderContentDisposition},
		AllowCredentials: false, // Set to false when using wildcard origins
		MaxAge:           86400, // Cache preflight for 24 hours
	}))

	// Health endpoints moved under /api prefix
	api.GET("/healthz", handlers.HealthHandler(s.config.Version))
	api.GET("/healthz/db", handlers.DBHealthHandler(s.db))

	// Swagger redirects (must be before wildcard route)
	s.echo.GET("/swagger", func(c echo.Context) error {
		return c.Redirect(301, "/swagger/index.html")
	})

	s.echo.GET("/swagger/", func(c echo.Context) error {
		return c.Redirect(301, "/swagger/index.html")
	})

	// Swagger documentation (must be before static files)
	s.echo.GET("/swagger/*", echoSwagger.WrapHandler)

	// API endpoints under /api prefix
	api.GET("/", handlers.RootHandler(s.config.Version))
	api.GET("/products", handlers.ProductsHandler(s.db))

	// Use vector-based chat if embedding service is available, otherwise fall back to regular chat
	if s.embeddingService != nil {
		api.POST("/chat", handlers.ChatVectorHandler(s.db, s.config, s.cache, s.embeddingService))
	} else {
		api.POST("/chat", handlers.ChatHandler(s.db, s.config, s.cache))
	}

	// Handle favicon requests
	s.echo.GET("/favicon.ico", func(c echo.Context) error {
		return c.NoContent(204) // No content response for favicon
	})

	// Serve static files for specific paths only
	s.echo.Static("/static", "static")
	s.echo.File("/", "static/index.html")
	s.echo.File("/index.html", "static/index.html")
	s.echo.File("/script.js", "static/script.js")
	s.echo.File("/style.css", "static/style.css")
}

// Start starts the HTTP server
func (s *Server) Start() error {
	s.logger.Info().Str("port", s.config.Port).Msg("Server starting")
	return s.echo.Start(":" + s.config.Port)
}
