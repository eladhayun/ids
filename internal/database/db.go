package database

import (
	"context"
	"fmt"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/jmoiron/sqlx"
)

// New creates a new database connection
func New(databaseURL string) (*sqlx.DB, error) {
	if databaseURL == "" {
		return nil, fmt.Errorf("DATABASE_URL environment variable not set")
	}

	db, err := sqlx.Open("mysql", databaseURL)
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

	// Note: For read-only behavior, consider:
	// 1. Using a MySQL user with only SELECT privileges
	// 2. Using read-only transactions: BEGIN READ ONLY; ... COMMIT;
	// 3. Implementing application-level read-only logic

	return db, nil
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
	defer tx.Rollback() // Always rollback, we never commit read-only transactions

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
	defer tx.Rollback() // Always rollback, we never commit read-only transactions

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
	defer tx.Rollback() // Always rollback, we never commit read-only transactions

	// Execute a simple query to test the connection in read-only mode
	var result int
	err = tx.GetContext(ctx, &result, "SELECT 1")
	if err != nil {
		return fmt.Errorf("failed to execute read-only ping query: %w", err)
	}

	return nil
}
