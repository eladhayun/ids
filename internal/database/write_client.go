package database

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	_ "github.com/go-sql-driver/mysql" // Keep for remote MySQL DB
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq" // Add for local PostgreSQL DB
)

// WriteClient provides write access to the database for embedding operations
type WriteClient struct {
	db *sqlx.DB
}

// NewWriteClient creates a new write-enabled database client (supports both MySQL and PostgreSQL)
func NewWriteClient(databaseURL string) (*WriteClient, error) {
	// Parse the URL to replace read-only user with write user
	writeURL := convertToWriteURL(databaseURL)

	// Auto-detect driver from URL
	driver := "mysql"
	if len(writeURL) > 8 && writeURL[:8] == "postgres" {
		driver = "postgres"
	}

	db, err := sqlx.Connect(driver, writeURL)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database with write access: %v", err)
	}

	// Configure connection pool
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(time.Hour)

	// Test the connection
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("failed to ping database: %v", err)
	}

	return &WriteClient{db: db}, nil
}

// GetDB returns the underlying database connection
func (wc *WriteClient) GetDB() *sqlx.DB {
	return wc.db
}

// ExecuteWriteQuery executes a write query and returns the result
func (wc *WriteClient) ExecuteWriteQuery(query string, args ...interface{}) (sql.Result, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	return wc.db.ExecContext(ctx, query, args...)
}

// ExecuteWriteQueryWithResult executes a write query and scans the result into dest
func (wc *WriteClient) ExecuteWriteQueryWithResult(dest interface{}, query string, args ...interface{}) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	return wc.db.SelectContext(ctx, dest, query, args...)
}

// ExecuteWriteQuerySingle executes a write query and scans a single result into dest
func (wc *WriteClient) ExecuteWriteQuerySingle(dest interface{}, query string, args ...interface{}) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	return wc.db.GetContext(ctx, dest, query, args...)
}

// Close closes the database connection
func (wc *WriteClient) Close() error {
	return wc.db.Close()
}

// convertToWriteURL converts a read-only database URL to a write-enabled URL
func convertToWriteURL(readOnlyURL string) string {
	// For now, we'll use the same URL but this could be enhanced to use different credentials
	// In production, you might want to use different database users with different permissions
	return readOnlyURL
}
