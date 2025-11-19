package database

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew_EmptyDatabaseURL(t *testing.T) {
	db, err := New("")
	assert.Error(t, err)
	assert.Nil(t, db)
	assert.Contains(t, err.Error(), "DATABASE_URL environment variable not set")
}

func TestExecuteReadOnlyQuery(t *testing.T) {
	tests := []struct {
		name      string
		setupMock func(mock sqlmock.Sqlmock)
		query     string
		wantError bool
		checkErr  func(t *testing.T, err error)
	}{
		{
			name: "successful query",
			setupMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectBegin()
				mock.ExpectQuery("SELECT \\* FROM users").
					WillReturnRows(sqlmock.NewRows([]string{"id", "name"}).
						AddRow(1, "John").
						AddRow(2, "Jane"))
				mock.ExpectRollback()
			},
			query:     "SELECT * FROM users",
			wantError: false,
		},
		{
			name: "transaction begin failure",
			setupMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectBegin().WillReturnError(sql.ErrConnDone)
			},
			query:     "SELECT * FROM users",
			wantError: true,
			checkErr: func(t *testing.T, err error) {
				assert.Contains(t, err.Error(), "failed to begin read-only transaction")
			},
		},
		{
			name: "query execution failure",
			setupMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectBegin()
				mock.ExpectQuery("SELECT \\* FROM users").
					WillReturnError(sql.ErrNoRows)
				mock.ExpectRollback()
			},
			query:     "SELECT * FROM users",
			wantError: true,
			checkErr: func(t *testing.T, err error) {
				assert.Contains(t, err.Error(), "failed to execute read-only query")
			},
		},
		{
			name: "empty result set",
			setupMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectBegin()
				mock.ExpectQuery("SELECT \\* FROM users").
					WillReturnRows(sqlmock.NewRows([]string{"id", "name"}))
				mock.ExpectRollback()
			},
			query:     "SELECT * FROM users",
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockDB, mock, err := sqlmock.New()
			require.NoError(t, err)
			defer mockDB.Close()

			db := sqlx.NewDb(mockDB, "sqlmock")
			tt.setupMock(mock)

			ctx := context.Background()
			var results []struct {
				ID   int    `db:"id"`
				Name string `db:"name"`
			}

			err = ExecuteReadOnlyQuery(ctx, db, &results, tt.query)

			if tt.wantError {
				assert.Error(t, err)
				if tt.checkErr != nil {
					tt.checkErr(t, err)
				}
			} else {
				assert.NoError(t, err)
			}

			err = mock.ExpectationsWereMet()
			assert.NoError(t, err)
		})
	}
}

func TestExecuteReadOnlyQuerySingle(t *testing.T) {
	tests := []struct {
		name      string
		setupMock func(mock sqlmock.Sqlmock)
		query     string
		args      []interface{}
		wantError bool
		checkErr  func(t *testing.T, err error)
	}{
		{
			name: "successful single row query",
			setupMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectBegin()
				mock.ExpectQuery("SELECT id FROM users WHERE id = \\?").
					WithArgs(1).
					WillReturnRows(sqlmock.NewRows([]string{"id"}).
						AddRow(1))
				mock.ExpectRollback()
			},
			query:     "SELECT id FROM users WHERE id = ?",
			args:      []interface{}{1},
			wantError: false,
		},
		{
			name: "no rows found",
			setupMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectBegin()
				mock.ExpectQuery("SELECT id, name FROM users WHERE id = \\?").
					WithArgs(999).
					WillReturnError(sql.ErrNoRows)
				mock.ExpectRollback()
			},
			query:     "SELECT id, name FROM users WHERE id = ?",
			args:      []interface{}{999},
			wantError: true,
			checkErr: func(t *testing.T, err error) {
				assert.Contains(t, err.Error(), "failed to execute read-only query")
			},
		},
		{
			name: "transaction begin failure",
			setupMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectBegin().WillReturnError(sql.ErrConnDone)
			},
			query:     "SELECT id, name FROM users WHERE id = ?",
			args:      []interface{}{1},
			wantError: true,
			checkErr: func(t *testing.T, err error) {
				assert.Contains(t, err.Error(), "failed to begin read-only transaction")
			},
		},
		{
			name: "simple SELECT 1",
			setupMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectBegin()
				mock.ExpectQuery("SELECT 1").
					WillReturnRows(sqlmock.NewRows([]string{"result"}).AddRow(1))
				mock.ExpectRollback()
			},
			query:     "SELECT 1",
			args:      nil,
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockDB, mock, err := sqlmock.New()
			require.NoError(t, err)
			defer mockDB.Close()

			db := sqlx.NewDb(mockDB, "sqlmock")
			tt.setupMock(mock)

			ctx := context.Background()
			var result int

			err = ExecuteReadOnlyQuerySingle(ctx, db, &result, tt.query, tt.args...)

			if tt.wantError {
				assert.Error(t, err)
				if tt.checkErr != nil {
					tt.checkErr(t, err)
				}
			} else {
				assert.NoError(t, err)
			}

			err = mock.ExpectationsWereMet()
			assert.NoError(t, err)
		})
	}
}

func TestExecuteReadOnlyPing(t *testing.T) {
	tests := []struct {
		name      string
		setupMock func(mock sqlmock.Sqlmock)
		wantError bool
		checkErr  func(t *testing.T, err error)
	}{
		{
			name: "successful ping",
			setupMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectBegin()
				mock.ExpectQuery("SELECT 1").
					WillReturnRows(sqlmock.NewRows([]string{"1"}).AddRow(1))
				mock.ExpectRollback()
			},
			wantError: false,
		},
		{
			name: "transaction begin failure",
			setupMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectBegin().WillReturnError(sql.ErrConnDone)
			},
			wantError: true,
			checkErr: func(t *testing.T, err error) {
				assert.Contains(t, err.Error(), "failed to begin read-only transaction")
			},
		},
		{
			name: "ping query failure",
			setupMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectBegin()
				mock.ExpectQuery("SELECT 1").
					WillReturnError(sql.ErrConnDone)
				mock.ExpectRollback()
			},
			wantError: true,
			checkErr: func(t *testing.T, err error) {
				assert.Contains(t, err.Error(), "failed to execute read-only ping query")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockDB, mock, err := sqlmock.New()
			require.NoError(t, err)
			defer mockDB.Close()

			db := sqlx.NewDb(mockDB, "sqlmock")
			tt.setupMock(mock)

			ctx := context.Background()
			err = ExecuteReadOnlyPing(ctx, db)

			if tt.wantError {
				assert.Error(t, err)
				if tt.checkErr != nil {
					tt.checkErr(t, err)
				}
			} else {
				assert.NoError(t, err)
			}

			err = mock.ExpectationsWereMet()
			assert.NoError(t, err)
		})
	}
}

func TestExecuteReadOnlyQuery_ContextCancellation(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer mockDB.Close()

	db := sqlx.NewDb(mockDB, "sqlmock")

	mock.ExpectBegin()
	mock.ExpectQuery("SELECT \\* FROM slow_table").
		WillDelayFor(2 * time.Second)
	mock.ExpectRollback()

	// Create context with short timeout
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	var results []struct {
		ID int `db:"id"`
	}

	err = ExecuteReadOnlyQuery(ctx, db, &results, "SELECT * FROM slow_table")
	assert.Error(t, err)
}

func TestExecuteReadOnlyQuerySingle_ContextCancellation(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer mockDB.Close()

	db := sqlx.NewDb(mockDB, "sqlmock")

	mock.ExpectBegin()
	mock.ExpectQuery("SELECT \\* FROM slow_table").
		WillDelayFor(2 * time.Second)
	mock.ExpectRollback()

	// Create context with short timeout
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	var result struct {
		ID int `db:"id"`
	}

	err = ExecuteReadOnlyQuerySingle(ctx, db, &result, "SELECT * FROM slow_table")
	assert.Error(t, err)
}

func TestExecuteReadOnlyPing_ContextCancellation(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer mockDB.Close()

	db := sqlx.NewDb(mockDB, "sqlmock")

	mock.ExpectBegin()
	mock.ExpectQuery("SELECT 1").
		WillDelayFor(2 * time.Second)
	mock.ExpectRollback()

	// Create context with short timeout
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	err = ExecuteReadOnlyPing(ctx, db)
	assert.Error(t, err)
}

func TestExecuteReadOnlyQuery_MultipleRows(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer mockDB.Close()

	db := sqlx.NewDb(mockDB, "sqlmock")

	// Expect query with multiple rows
	rows := sqlmock.NewRows([]string{"id", "name"})
	for i := 1; i <= 100; i++ {
		rows.AddRow(i, "User"+string(rune(i)))
	}

	mock.ExpectBegin()
	mock.ExpectQuery("SELECT id, name FROM users").WillReturnRows(rows)
	mock.ExpectRollback()

	ctx := context.Background()
	var results []struct {
		ID   int    `db:"id"`
		Name string `db:"name"`
	}

	err = ExecuteReadOnlyQuery(ctx, db, &results, "SELECT id, name FROM users")
	assert.NoError(t, err)
	assert.Len(t, results, 100)

	err = mock.ExpectationsWereMet()
	assert.NoError(t, err)
}

func BenchmarkExecuteReadOnlyQuery(b *testing.B) {
	mockDB, mock, err := sqlmock.New()
	require.NoError(b, err)
	defer mockDB.Close()

	db := sqlx.NewDb(mockDB, "sqlmock")

	for i := 0; i < b.N; i++ {
		mock.ExpectBegin()
		mock.ExpectQuery("SELECT 1").
			WillReturnRows(sqlmock.NewRows([]string{"1"}).AddRow(1))
		mock.ExpectRollback()
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ctx := context.Background()
		var results []struct {
			Val int `db:"1"`
		}
		ExecuteReadOnlyQuery(ctx, db, &results, "SELECT 1")
	}
}

func BenchmarkExecuteReadOnlyQuerySingle(b *testing.B) {
	mockDB, mock, err := sqlmock.New()
	require.NoError(b, err)
	defer mockDB.Close()

	db := sqlx.NewDb(mockDB, "sqlmock")

	for i := 0; i < b.N; i++ {
		mock.ExpectBegin()
		mock.ExpectQuery("SELECT 1").
			WillReturnRows(sqlmock.NewRows([]string{"1"}).AddRow(1))
		mock.ExpectRollback()
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ctx := context.Background()
		var result struct {
			Val int `db:"1"`
		}
		ExecuteReadOnlyQuerySingle(ctx, db, &result, "SELECT 1")
	}
}
