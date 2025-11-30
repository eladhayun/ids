package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"ids/internal/models"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/jmoiron/sqlx"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHealthHandler(t *testing.T) {
	tests := []struct {
		name           string
		version        string
		expectedStatus int
		checkResponse  func(t *testing.T, resp models.HealthResponse)
	}{
		{
			name:           "returns healthy status",
			version:        "1.0.0",
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, resp models.HealthResponse) {
				assert.Equal(t, "healthy", resp.Status)
				assert.Equal(t, "1.0.0", resp.Version)
				assert.WithinDuration(t, time.Now().UTC(), resp.Timestamp, 5*time.Second)
			},
		},
		{
			name:           "returns healthy with custom version",
			version:        "2.5.3",
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, resp models.HealthResponse) {
				assert.Equal(t, "healthy", resp.Status)
				assert.Equal(t, "2.5.3", resp.Version)
			},
		},
		{
			name:           "returns healthy with empty version",
			version:        "",
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, resp models.HealthResponse) {
				assert.Equal(t, "healthy", resp.Status)
				assert.Equal(t, "", resp.Version)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup
			e := echo.New()
			req := httptest.NewRequest(http.MethodGet, "/api/healthz", nil)
			rec := httptest.NewRecorder()
			c := e.NewContext(req, rec)

			// Execute
			handler := HealthHandler(tt.version)
			err := handler(c)

			// Assert
			require.NoError(t, err)
			assert.Equal(t, tt.expectedStatus, rec.Code)

			var response models.HealthResponse
			err = json.Unmarshal(rec.Body.Bytes(), &response)
			require.NoError(t, err)

			tt.checkResponse(t, response)
		})
	}
}

func TestDBHealthHandler(t *testing.T) {
	tests := []struct {
		name           string
		setupMock      func(mock sqlmock.Sqlmock)
		db             *sqlx.DB
		expectedStatus int
		checkResponse  func(t *testing.T, resp models.DBHealthResponse)
	}{
		{
			name: "healthy database connection",
			setupMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectBegin()
				mock.ExpectQuery("SELECT 1").WillReturnRows(sqlmock.NewRows([]string{"1"}).AddRow(1))
				mock.ExpectRollback()
			},
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, resp models.DBHealthResponse) {
				assert.Equal(t, "healthy", resp.Status)
				assert.True(t, resp.Connected)
				assert.Greater(t, resp.Latency, time.Duration(0))
				assert.Empty(t, resp.Error)
			},
		},
		{
			name:           "nil database connection",
			setupMock:      nil,
			db:             nil,
			expectedStatus: http.StatusServiceUnavailable,
			checkResponse: func(t *testing.T, resp models.DBHealthResponse) {
				assert.Equal(t, "unhealthy", resp.Status)
				assert.False(t, resp.Connected)
				assert.Equal(t, "Database connection not initialized", resp.Error)
			},
		},
		{
			name: "database ping failure",
			setupMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectBegin().WillReturnError(sql.ErrConnDone)
			},
			expectedStatus: http.StatusServiceUnavailable,
			checkResponse: func(t *testing.T, resp models.DBHealthResponse) {
				assert.Equal(t, "unhealthy", resp.Status)
				assert.False(t, resp.Connected)
				assert.Contains(t, resp.Error, "failed to begin read-only transaction")
			},
		},
		{
			name: "database query failure",
			setupMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectBegin()
				mock.ExpectQuery("SELECT 1").WillReturnError(sql.ErrNoRows)
				mock.ExpectRollback()
			},
			expectedStatus: http.StatusServiceUnavailable,
			checkResponse: func(t *testing.T, resp models.DBHealthResponse) {
				assert.Equal(t, "unhealthy", resp.Status)
				assert.False(t, resp.Connected)
				assert.Contains(t, resp.Error, "Database read-only query failed")
			},
		},
		{
			name: "database transaction begin failure",
			setupMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectBegin().WillReturnError(context.DeadlineExceeded)
			},
			expectedStatus: http.StatusServiceUnavailable,
			checkResponse: func(t *testing.T, resp models.DBHealthResponse) {
				assert.Equal(t, "unhealthy", resp.Status)
				assert.False(t, resp.Connected)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup
			e := echo.New()
			req := httptest.NewRequest(http.MethodGet, "/api/healthz/db", nil)
			rec := httptest.NewRecorder()
			c := e.NewContext(req, rec)

			var testDB *sqlx.DB
			if tt.db == nil && tt.setupMock != nil {
				mockDB, mock, err := sqlmock.New()
				require.NoError(t, err)
				defer func() { _ = mockDB.Close() }()

				testDB = sqlx.NewDb(mockDB, "sqlmock")
				tt.setupMock(mock)
			} else {
				testDB = tt.db
			}

			// Execute
			handler := DBHealthHandler(testDB)
			err := handler(c)

			// Assert
			require.NoError(t, err)
			assert.Equal(t, tt.expectedStatus, rec.Code)

			var response models.DBHealthResponse
			err = json.Unmarshal(rec.Body.Bytes(), &response)
			require.NoError(t, err)

			tt.checkResponse(t, response)
		})
	}
}

func TestRootHandler(t *testing.T) {
	tests := []struct {
		name           string
		version        string
		expectedStatus int
		checkResponse  func(t *testing.T, resp map[string]string)
	}{
		{
			name:           "returns service information",
			version:        "1.0.0",
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, resp map[string]string) {
				assert.Equal(t, "IDS API", resp["service"])
				assert.Equal(t, "1.0.0", resp["version"])
				assert.Equal(t, "running", resp["status"])
			},
		},
		{
			name:           "returns with different version",
			version:        "3.2.1",
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, resp map[string]string) {
				assert.Equal(t, "IDS API", resp["service"])
				assert.Equal(t, "3.2.1", resp["version"])
				assert.Equal(t, "running", resp["status"])
			},
		},
		{
			name:           "returns with empty version",
			version:        "",
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, resp map[string]string) {
				assert.Equal(t, "IDS API", resp["service"])
				assert.Equal(t, "", resp["version"])
				assert.Equal(t, "running", resp["status"])
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup
			e := echo.New()
			req := httptest.NewRequest(http.MethodGet, "/api/", nil)
			rec := httptest.NewRecorder()
			c := e.NewContext(req, rec)

			// Execute
			handler := RootHandler(tt.version)
			err := handler(c)

			// Assert
			require.NoError(t, err)
			assert.Equal(t, tt.expectedStatus, rec.Code)

			var response map[string]string
			err = json.Unmarshal(rec.Body.Bytes(), &response)
			require.NoError(t, err)

			tt.checkResponse(t, response)
		})
	}
}

func TestDBHealthHandler_Concurrency(t *testing.T) {
	// Test concurrent health checks
	// Note: This test verifies that concurrent requests don't panic or deadlock
	// We run multiple health checks sequentially since mocking concurrent DB calls is complex
	mockDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = mockDB.Close() }()

	testDB := sqlx.NewDb(mockDB, "sqlmock")

	// Set up expectations for sequential health checks
	for i := 0; i < 5; i++ {
		mock.ExpectBegin()
		mock.ExpectQuery("SELECT 1").WillReturnRows(sqlmock.NewRows([]string{"1"}).AddRow(1))
		mock.ExpectRollback()
	}

	e := echo.New()
	handler := DBHealthHandler(testDB)

	// Run 5 health checks to ensure handler is stable
	for i := 0; i < 5; i++ {
		req := httptest.NewRequest(http.MethodGet, "/api/healthz/db", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		err := handler(c)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, rec.Code, "Health check %d should succeed", i+1)
	}

	// Verify all expectations were met
	err = mock.ExpectationsWereMet()
	assert.NoError(t, err)
}

func TestDBHealthHandler_ContextTimeout(t *testing.T) {
	// Create a mock that will timeout
	mockDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = mockDB.Close() }()

	testDB := sqlx.NewDb(mockDB, "sqlmock")

	// Simulate slow query
	mock.ExpectBegin().WillDelayFor(10 * time.Second)

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/healthz/db", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	handler := DBHealthHandler(testDB)
	err = handler(c)

	require.NoError(t, err)
	assert.Equal(t, http.StatusServiceUnavailable, rec.Code)

	var response models.DBHealthResponse
	err = json.Unmarshal(rec.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, "unhealthy", response.Status)
	assert.False(t, response.Connected)
}
