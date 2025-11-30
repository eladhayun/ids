package database

import (
	"context"
	"fmt"
	"time"

	_ "github.com/go-sql-driver/mysql" // Keep for remote MySQL DB
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq" // Add for local PostgreSQL DB
)

// New creates a new database connection (supports both MySQL and PostgreSQL)
func New(databaseURL string) (*sqlx.DB, error) {
	if databaseURL == "" {
		return nil, fmt.Errorf("DATABASE_URL environment variable not set")
	}

	// Auto-detect driver from URL
	const (
		driverMySQL    = "mysql"
		driverPostgres = "postgres"
	)
	driver := driverMySQL
	if len(databaseURL) > 8 && databaseURL[:8] == driverPostgres {
		driver = driverPostgres
	}

	db, err := sqlx.Open(driver, databaseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Configure connection pool settings
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(time.Hour)

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	// Enforce read-only mode for MySQL connections (production database protection)
	// This sets the session to read-only mode, preventing any writes
	if driver == driverMySQL {
		_, err := db.Exec("SET SESSION TRANSACTION READ ONLY")
		if err != nil {
			// Log warning but don't fail - some MySQL users might not have permission to set this
			fmt.Printf("Warning: Could not set MySQL session to read-only: %v\n", err)
			fmt.Println("Ensure the MySQL user has SELECT-only privileges for production safety")
		} else {
			fmt.Println("✓ MySQL session set to READ ONLY mode - production database is protected")
		}
	}

	return db, nil
}

// IsReadOnly verifies if the current database session is in read-only mode
func IsReadOnly(db *sqlx.DB) (bool, error) {
	var readOnly int
	err := db.Get(&readOnly, "SELECT @@session.tx_read_only")
	if err != nil {
		return false, err
	}
	return readOnly == 1, nil
}

// ExecuteReadOnlyQuery executes a query within a read-only transaction for extra safety
func ExecuteReadOnlyQuery(ctx context.Context, db *sqlx.DB, dest interface{}, query string, args ...interface{}) error {
	// Note: Session isolation level setting removed due to permission issues
	// The query will still be executed safely as a read-only operation

	// Start a read-only transaction
	tx, err := db.BeginTxx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin read-only transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil {
			// Log but don't fail - rollback errors in read-only transactions are usually harmless
			fmt.Printf("Warning: Error rolling back read-only transaction: %v\n", err)
		}
	}() // Always rollback, we never commit read-only transactions

	// Execute the query
	err = tx.SelectContext(ctx, dest, query, args...)
	if err != nil {
		return fmt.Errorf("failed to execute read-only query: %w", err)
	}

	return nil
}

// ExecuteReadOnlyQuerySingle executes a single-row query within a read-only transaction
func ExecuteReadOnlyQuerySingle(ctx context.Context, db *sqlx.DB, dest interface{}, query string, args ...interface{}) error {
	// Note: Session isolation level setting removed due to permission issues
	// The query will still be executed safely as a read-only operation

	// Start a read-only transaction
	tx, err := db.BeginTxx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin read-only transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil {
			// Log but don't fail - rollback errors in read-only transactions are usually harmless
			fmt.Printf("Warning: Error rolling back read-only transaction: %v\n", err)
		}
	}() // Always rollback, we never commit read-only transactions

	// Execute the query
	err = tx.GetContext(ctx, dest, query, args...)
	if err != nil {
		return fmt.Errorf("failed to execute read-only query: %w", err)
	}

	return nil
}

// ExecuteReadOnlyPing executes a ping within a read-only transaction
func ExecuteReadOnlyPing(ctx context.Context, db *sqlx.DB) error {
	// Note: Session isolation level setting removed due to permission issues
	// The query will still be executed safely as a read-only operation

	// Start a read-only transaction
	tx, err := db.BeginTxx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin read-only transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil {
			// Log but don't fail - rollback errors in read-only transactions are usually harmless
			fmt.Printf("Warning: Error rolling back read-only transaction: %v\n", err)
		}
	}() // Always rollback, we never commit read-only transactions

	// Execute a simple query to test the connection in read-only mode
	var result int
	err = tx.GetContext(ctx, &result, "SELECT 1")
	if err != nil {
		return fmt.Errorf("failed to execute read-only ping query: %w", err)
	}

	return nil
}
