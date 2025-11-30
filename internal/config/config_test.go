package config

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLoad_DefaultValues(t *testing.T) {
	// Clear environment variables
	clearEnv(t)

	cfg := Load()

	assert.Equal(t, "8080", cfg.Port)
	assert.Equal(t, "1.0.0", cfg.Version)
	assert.Equal(t, "info", cfg.LogLevel)
	assert.True(t, cfg.WaitForTunnel)
	assert.Equal(t, 60, cfg.OpenAITimeout)
	assert.Equal(t, 168, cfg.EmbeddingScheduleHours)
}

func TestLoad_CustomValues(t *testing.T) {
	// Set environment variables
	clearEnv(t)
	_ = os.Setenv("PORT", "9090")
	_ = os.Setenv("DATABASE_URL", "mysql://user:pass@localhost:3306/testdb")
	_ = os.Setenv("VERSION", "2.0.0")
	_ = os.Setenv("LOG_LEVEL", "debug")
	_ = os.Setenv("OPENAI_API_KEY", "test-key-123")
	_ = os.Setenv("WAIT_FOR_TUNNEL", "false")
	_ = os.Setenv("OPENAI_TIMEOUT", "120")
	_ = os.Setenv("EMBEDDING_SCHEDULE_INTERVAL_HOURS", "24")

	cfg := Load()

	assert.Equal(t, "9090", cfg.Port)
	assert.Equal(t, "mysql://user:pass@localhost:3306/testdb", cfg.DatabaseURL)
	assert.Equal(t, "2.0.0", cfg.Version)
	assert.Equal(t, "debug", cfg.LogLevel)
	assert.Equal(t, "test-key-123", cfg.OpenAIKey)
	assert.False(t, cfg.WaitForTunnel)
	assert.Equal(t, 120, cfg.OpenAITimeout)
	assert.Equal(t, 24, cfg.EmbeddingScheduleHours)
}

func TestLoad_PartialCustomValues(t *testing.T) {
	clearEnv(t)
	_ = os.Setenv("PORT", "3000")
	_ = os.Setenv("OPENAI_API_KEY", "sk-test")

	cfg := Load()

	// Custom values
	assert.Equal(t, "3000", cfg.Port)
	assert.Equal(t, "sk-test", cfg.OpenAIKey)

	// Default values for unset variables
	assert.Equal(t, "1.0.0", cfg.Version)
	assert.Equal(t, "info", cfg.LogLevel)
	assert.True(t, cfg.WaitForTunnel)
	assert.Equal(t, 60, cfg.OpenAITimeout)
	assert.Equal(t, 168, cfg.EmbeddingScheduleHours)
}

func TestGetEnv(t *testing.T) {
	tests := []struct {
		name         string
		key          string
		value        string
		defaultValue string
		expected     string
	}{
		{
			name:         "existing value",
			key:          "TEST_KEY",
			value:        "test_value",
			defaultValue: "default",
			expected:     "test_value",
		},
		{
			name:         "missing value uses default",
			key:          "MISSING_KEY",
			value:        "",
			defaultValue: "default",
			expected:     "default",
		},
		{
			name:         "empty value uses default",
			key:          "EMPTY_KEY",
			value:        "",
			defaultValue: "default",
			expected:     "default",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.value != "" {
				_ = os.Setenv(tt.key, tt.value)
				defer func() { _ = os.Unsetenv(tt.key) }()
			}

			result := getEnv(tt.key, tt.defaultValue)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetEnvInt(t *testing.T) {
	tests := []struct {
		name         string
		key          string
		value        string
		defaultValue int
		expected     int
	}{
		{
			name:         "valid integer",
			key:          "TEST_INT",
			value:        "42",
			defaultValue: 10,
			expected:     42,
		},
		{
			name:         "zero value",
			key:          "TEST_ZERO",
			value:        "0",
			defaultValue: 10,
			expected:     0,
		},
		{
			name:         "negative value",
			key:          "TEST_NEGATIVE",
			value:        "-5",
			defaultValue: 10,
			expected:     -5,
		},
		{
			name:         "invalid value uses default",
			key:          "TEST_INVALID",
			value:        "not-a-number",
			defaultValue: 10,
			expected:     10,
		},
		{
			name:         "missing value uses default",
			key:          "TEST_MISSING",
			value:        "",
			defaultValue: 10,
			expected:     10,
		},
		{
			name:         "large number",
			key:          "TEST_LARGE",
			value:        "999999",
			defaultValue: 10,
			expected:     999999,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.value != "" {
				_ = os.Setenv(tt.key, tt.value)
				defer func() { _ = os.Unsetenv(tt.key) }()
			}

			result := getEnvInt(tt.key, tt.defaultValue)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetEnvBool(t *testing.T) {
	tests := []struct {
		name         string
		key          string
		value        string
		defaultValue bool
		expected     bool
	}{
		{
			name:         "true value",
			key:          "TEST_TRUE",
			value:        "true",
			defaultValue: false,
			expected:     true,
		},
		{
			name:         "false value",
			key:          "TEST_FALSE",
			value:        "false",
			defaultValue: true,
			expected:     false,
		},
		{
			name:         "1 as true",
			key:          "TEST_ONE",
			value:        "1",
			defaultValue: false,
			expected:     true,
		},
		{
			name:         "0 as false",
			key:          "TEST_ZERO",
			value:        "0",
			defaultValue: true,
			expected:     false,
		},
		{
			name:         "invalid value uses default",
			key:          "TEST_INVALID",
			value:        "not-a-bool",
			defaultValue: true,
			expected:     true,
		},
		{
			name:         "missing value uses default",
			key:          "TEST_MISSING",
			value:        "",
			defaultValue: false,
			expected:     false,
		},
		{
			name:         "case insensitive TRUE",
			key:          "TEST_UPPER_TRUE",
			value:        "TRUE",
			defaultValue: false,
			expected:     true,
		},
		{
			name:         "case insensitive FALSE",
			key:          "TEST_UPPER_FALSE",
			value:        "FALSE",
			defaultValue: true,
			expected:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.value != "" {
				_ = os.Setenv(tt.key, tt.value)
				defer func() { _ = os.Unsetenv(tt.key) }()
			}

			result := getEnvBool(tt.key, tt.defaultValue)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestSetupLogger(t *testing.T) {
	tests := []struct {
		name     string
		logLevel string
	}{
		{"debug level", "debug"},
		{"info level", "info"},
		{"warn level", "warn"},
		{"error level", "error"},
		{"invalid level defaults to info", "invalid"},
		{"empty level defaults to info", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{
				Version:  "test-version",
				LogLevel: tt.logLevel,
			}

			logger := cfg.SetupLogger()
			assert.NotNil(t, logger)
		})
	}
}

func TestLoad_EmptyDatabaseURL(t *testing.T) {
	clearEnv(t)
	_ = os.Unsetenv("DATABASE_URL")

	cfg := Load()
	assert.Empty(t, cfg.DatabaseURL)
}

func TestLoad_EmptyOpenAIKey(t *testing.T) {
	clearEnv(t)
	_ = os.Unsetenv("OPENAI_API_KEY")

	cfg := Load()
	assert.Empty(t, cfg.OpenAIKey)
}

func TestLoad_EdgeCaseValues(t *testing.T) {
	clearEnv(t)

	// Test very large timeout
	_ = os.Setenv("OPENAI_TIMEOUT", "999999")
	_ = os.Setenv("EMBEDDING_SCHEDULE_INTERVAL_HOURS", "999999")

	cfg := Load()
	assert.Equal(t, 999999, cfg.OpenAITimeout)
	assert.Equal(t, 999999, cfg.EmbeddingScheduleHours)

	// Test zero values
	_ = os.Setenv("OPENAI_TIMEOUT", "0")
	_ = os.Setenv("EMBEDDING_SCHEDULE_INTERVAL_HOURS", "0")

	cfg = Load()
	assert.Equal(t, 0, cfg.OpenAITimeout)
	assert.Equal(t, 0, cfg.EmbeddingScheduleHours)
}

func TestLoad_SpecialCharacters(t *testing.T) {
	clearEnv(t)

	// Test special characters in values
	_ = os.Setenv("DATABASE_URL", "mysql://user:p@$$w0rd!@localhost:3306/db?charset=utf8mb4")
	_ = os.Setenv("OPENAI_API_KEY", "sk-test_key-123!@#$%")

	cfg := Load()
	assert.Equal(t, "mysql://user:p@$$w0rd!@localhost:3306/db?charset=utf8mb4", cfg.DatabaseURL)
	assert.Equal(t, "sk-test_key-123!@#$%", cfg.OpenAIKey)
}

func TestConfig_Struct(t *testing.T) {
	cfg := &Config{
		Port:                   "8080",
		DatabaseURL:            "mysql://localhost",
		Version:                "1.0.0",
		LogLevel:               "info",
		OpenAIKey:              "test-key",
		WaitForTunnel:          true,
		OpenAITimeout:          60,
		EmbeddingScheduleHours: 168,
	}

	assert.Equal(t, "8080", cfg.Port)
	assert.Equal(t, "mysql://localhost", cfg.DatabaseURL)
	assert.Equal(t, "1.0.0", cfg.Version)
	assert.Equal(t, "info", cfg.LogLevel)
	assert.Equal(t, "test-key", cfg.OpenAIKey)
	assert.True(t, cfg.WaitForTunnel)
	assert.Equal(t, 60, cfg.OpenAITimeout)
	assert.Equal(t, 168, cfg.EmbeddingScheduleHours)
}

// Helper function to clear relevant environment variables
func clearEnv(t *testing.T) {
	vars := []string{
		"PORT",
		"DATABASE_URL",
		"VERSION",
		"LOG_LEVEL",
		"OPENAI_API_KEY",
		"WAIT_FOR_TUNNEL",
		"OPENAI_TIMEOUT",
		"EMBEDDING_SCHEDULE_INTERVAL_HOURS",
	}

	for _, v := range vars {
		_ = os.Unsetenv(v)
	}

	// Cleanup after test
	t.Cleanup(func() {
		for _, v := range vars {
			_ = os.Unsetenv(v)
		}
	})
}

func BenchmarkLoad(b *testing.B) {
	for i := 0; i < b.N; i++ {
		Load()
	}
}

func BenchmarkSetupLogger(b *testing.B) {
	cfg := &Config{
		Version:  "1.0.0",
		LogLevel: "info",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cfg.SetupLogger()
	}
}
