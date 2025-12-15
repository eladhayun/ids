package handlers

import (
	"fmt"
	"net/http"

	"ids/internal/analytics"
	"ids/internal/models"

	"github.com/labstack/echo/v4"
)

// AnalyticsHandler returns analytics summary for a given period
// @Summary Get analytics summary
// @Description Get analytics summary for a specified time period (today, yesterday, last_7_days, last_30_days)
// @Tags analytics
// @Accept json
// @Produce json
// @Param period query string false "Time period (today, yesterday, last_7_days, last_30_days)" default(yesterday)
// @Success 200 {object} models.AnalyticsResponse
// @Failure 500 {object} models.AnalyticsResponse
// @Router /api/analytics [get]
func AnalyticsHandler(analyticsService *analytics.Service) echo.HandlerFunc {
	return func(c echo.Context) error {
		period := c.QueryParam("period")
		if period == "" {
			period = "yesterday"
		}

		fmt.Printf("[ANALYTICS] Fetching analytics summary for period: %s\n", period)

		summary, err := analyticsService.GetSummary(period)
		if err != nil {
			fmt.Printf("[ANALYTICS] ERROR: Failed to get analytics summary: %v\n", err)
			return c.JSON(http.StatusInternalServerError, models.AnalyticsResponse{
				Success: false,
				Error:   fmt.Sprintf("Failed to get analytics summary: %v", err),
			})
		}

		fmt.Printf("[ANALYTICS] ✅ Analytics summary retrieved successfully\n")
		return c.JSON(http.StatusOK, models.AnalyticsResponse{
			Success: true,
			Summary: summary,
		})
	}
}

// DailyReportHandler returns the daily analytics report (used by slack-notifications)
// @Summary Get daily analytics report
// @Description Get analytics report for the previous day, suitable for daily Slack notifications
// @Tags analytics
// @Accept json
// @Produce json
// @Success 200 {object} models.AnalyticsResponse
// @Failure 500 {object} models.AnalyticsResponse
// @Router /api/analytics/daily-report [get]
func DailyReportHandler(analyticsService *analytics.Service) echo.HandlerFunc {
	return func(c echo.Context) error {
		fmt.Printf("[ANALYTICS] Generating daily report for Slack notification\n")

		summary, err := analyticsService.GetDailyReport()
		if err != nil {
			fmt.Printf("[ANALYTICS] ERROR: Failed to generate daily report: %v\n", err)
			return c.JSON(http.StatusInternalServerError, models.AnalyticsResponse{
				Success: false,
				Error:   fmt.Sprintf("Failed to generate daily report: %v", err),
			})
		}

		fmt.Printf("[ANALYTICS] ✅ Daily report generated successfully\n")
		fmt.Printf("[ANALYTICS] Summary: Conversations=%d, Products=%d, Support=%d, OpenAI Calls=%d, Tokens=%d\n",
			summary.TotalConversations,
			summary.ProductSuggestions,
			summary.SupportEscalations,
			summary.OpenAICalls,
			summary.OpenAITokensUsed,
		)

		return c.JSON(http.StatusOK, models.AnalyticsResponse{
			Success: true,
			Summary: summary,
		})
	}
}
