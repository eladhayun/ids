package config

import (
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
	"github.com/rs/zerolog"
)

// Config holds all configuration for the application
type Config struct {
	Port                   string
	DatabaseURL            string // Remote database (via SSH tunnel) - read-only for product data
	EmbeddingsDatabaseURL  string // Local MariaDB - for storing embeddings and email data
	Version                string
	LogLevel               string
	OpenAIKey              string
	WaitForTunnel          bool   // Whether to wait for SSH tunnel to be ready
	OpenAITimeout          int    // OpenAI API timeout in seconds
	EmbeddingScheduleHours int    // Embedding generation schedule interval in hours
	EnableEmailContext     bool   // Whether to include email history in chat responses
	SendGridAPIKey         string // SendGrid API key for sending support escalation emails
	SupportEmail           string // Support email address (default: support@israeldefensestore.com)
}

// Load initializes and returns application configuration
func Load() *Config {
	// Load environment variables from .env file
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using environment variables")
	}

	config := &Config{
		Port:                   getEnv("PORT", "8080"),
		DatabaseURL:            os.Getenv("DATABASE_URL"),            // Remote DB via SSH
		EmbeddingsDatabaseURL:  os.Getenv("EMBEDDINGS_DATABASE_URL"), // Local MariaDB
		Version:                getEnv("VERSION", "1.0.0"),
		LogLevel:               getEnv("LOG_LEVEL", "info"),
		OpenAIKey:              os.Getenv("OPENAI_API_KEY"),
		WaitForTunnel:          getEnvBool("WAIT_FOR_TUNNEL", true),                       // Default true for production safety
		OpenAITimeout:          getEnvInt("OPENAI_TIMEOUT", 60),                           // Default 60 seconds
		EmbeddingScheduleHours: getEnvInt("EMBEDDING_SCHEDULE_INTERVAL_HOURS", 168),       // Default 168 hours (1 week)
		EnableEmailContext:     getEnvBool("ENABLE_EMAIL_CONTEXT", true),                  // Default true to use email history
		SendGridAPIKey:         os.Getenv("SENDGRID_API_KEY"),                             // SendGrid API key for support emails
		SupportEmail:           getEnv("SUPPORT_EMAIL", "support@israeldefensestore.com"), // Support email address
	}

	return config
}

// getEnv gets an environment variable with a default fallback
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// getEnvInt gets an environment variable as integer with a default fallback
func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}

// getEnvBool gets an environment variable as boolean with a default fallback
func getEnvBool(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		if boolValue, err := strconv.ParseBool(value); err == nil {
			return boolValue
		}
	}
	return defaultValue
}

// SetupLogger configures zerolog with JSON output and single-line format
func (c *Config) SetupLogger() zerolog.Logger {
	// Configure zerolog to output JSON without newlines
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix

	// Create logger with JSON output to stdout
	logger := zerolog.New(os.Stdout).With().
		Timestamp().
		Str("service", "ids").
		Str("version", c.Version).
		Logger()

	// Set log level based on configuration
	level, err := zerolog.ParseLevel(strings.ToLower(c.LogLevel))
	if err != nil {
		level = zerolog.InfoLevel
	}
	logger = logger.Level(level)

	return logger
}
