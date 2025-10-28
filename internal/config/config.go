package config

import (
	"log"
	"os"
	"strings"

	"github.com/joho/godotenv"
	"github.com/rs/zerolog"
)

// Config holds all configuration for the application
type Config struct {
	Port        string
	DatabaseURL string
	Version     string
	LogLevel    string
}

// Load initializes and returns application configuration
func Load() *Config {
	// Load environment variables from .env file
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using environment variables")
	}

	config := &Config{
		Port:        getEnv("PORT", "8080"),
		DatabaseURL: os.Getenv("DATABASE_URL"),
		Version:     getEnv("VERSION", "1.0.0"),
		LogLevel:    getEnv("LOG_LEVEL", "info"),
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
