package handlers

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"time"

	"ids/internal/config"
	"ids/internal/k8s"

	"github.com/labstack/echo/v4"
)

// TriggerEmailImportRequest represents the request to trigger email import
type TriggerEmailImportRequest struct {
	Source string `json:"source"` // Optional: specific source/path in blob storage
}

// TriggerEmailImportResponse represents the response from triggering email import
type TriggerEmailImportResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	JobName string `json:"job_name,omitempty"`
	Error   string `json:"error,omitempty"`
}

// JobStatus represents the status of a Kubernetes job
type JobStatus struct {
	JobName        string  `json:"job_name"`
	Status         string  `json:"status"`
	Active         int32   `json:"active"`
	Succeeded      int32   `json:"succeeded"`
	Failed         int32   `json:"failed"`
	StartTime      *string `json:"start_time,omitempty"`
	CompletionTime *string `json:"completion_time,omitempty"`
}

// TriggerEmailImportHandler triggers a Kubernetes Job to import emails from Azure Blob Storage
// @Summary Trigger email import job
// @Description Triggers a Kubernetes Job that downloads emails from Azure Blob Storage and imports them into the database
// @Tags admin
// @Accept json
// @Produce json
// @Param request body TriggerEmailImportRequest false "Import job parameters"
// @Success 200 {object} TriggerEmailImportResponse
// @Failure 400 {object} TriggerEmailImportResponse
// @Failure 500 {object} TriggerEmailImportResponse
// @Router /api/admin/trigger-email-import [post]
func TriggerEmailImportHandler(cfg *config.Config) echo.HandlerFunc {
	return func(c echo.Context) error {
		fmt.Println("[EMAIL_IMPORT_JOB] Received trigger request")

		var req TriggerEmailImportRequest
		if err := c.Bind(&req); err != nil {
			fmt.Printf("[EMAIL_IMPORT_JOB] Invalid request: %v\n", err)
			return c.JSON(http.StatusBadRequest, TriggerEmailImportResponse{
				Success: false,
				Error:   "Invalid request body",
			})
		}

		// Generate unique job name with timestamp
		jobName := fmt.Sprintf("email-import-%d", time.Now().Unix())

		// Create Kubernetes client for ids namespace
		k8sClient, err := k8s.NewClient("ids")
		if err != nil {
			fmt.Printf("[EMAIL_IMPORT_JOB] Failed to create Kubernetes client: %v\n", err)
			return c.JSON(http.StatusInternalServerError, TriggerEmailImportResponse{
				Success: false,
				Error:   fmt.Sprintf("Failed to create Kubernetes client: %v", err),
			})
		}

		// Create Kubernetes Job
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		// Use the correct image name from environment or default to the production image
		containerImage := os.Getenv("EMAIL_IMPORT_IMAGE")
		if containerImage == "" {
			containerImage = "prodacr1234.azurecr.io/ids:latest"
		}

		if err := k8sClient.CreateEmailImportJob(ctx, jobName, containerImage); err != nil {
			fmt.Printf("[EMAIL_IMPORT_JOB] Failed to create job: %v\n", err)
			return c.JSON(http.StatusInternalServerError, TriggerEmailImportResponse{
				Success: false,
				Error:   fmt.Sprintf("Failed to create Kubernetes job: %v", err),
			})
		}

		fmt.Printf("[EMAIL_IMPORT_JOB] Successfully created job: %s\n", jobName)

		return c.JSON(http.StatusOK, TriggerEmailImportResponse{
			Success: true,
			Message: "Email import job triggered successfully",
			JobName: jobName,
		})
	}
}

// GetEmailImportStatusHandler gets the status of an email import job
// @Summary Get email import job status
// @Description Gets the current status of an email import job
// @Tags admin
// @Accept json
// @Produce json
// @Param jobName path string true "Job name"
// @Success 200 {object} JobStatus
// @Failure 404 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /api/admin/email-import-status/{jobName} [get]
func GetEmailImportStatusHandler(cfg *config.Config) echo.HandlerFunc {
	return func(c echo.Context) error {
		jobName := c.Param("jobName")

		fmt.Printf("[EMAIL_IMPORT_JOB] Getting status for job: %s\n", jobName)

		// Create Kubernetes client for ids namespace
		k8sClient, err := k8s.NewClient("ids")
		if err != nil {
			fmt.Printf("[EMAIL_IMPORT_JOB] Failed to create Kubernetes client: %v\n", err)
			return c.JSON(http.StatusInternalServerError, map[string]string{
				"error": fmt.Sprintf("Failed to create Kubernetes client: %v", err),
			})
		}

		// Get job status
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		job, err := k8sClient.GetJobStatus(ctx, jobName)
		if err != nil {
			fmt.Printf("[EMAIL_IMPORT_JOB] Failed to get job status: %v\n", err)
			return c.JSON(http.StatusNotFound, map[string]string{
				"error": fmt.Sprintf("Job not found: %v", err),
			})
		}

		// Determine status
		status := "pending"
		if job.Status.Active > 0 {
			status = "running"
		} else if job.Status.Succeeded > 0 {
			status = "completed"
		} else if job.Status.Failed > 0 {
			status = "failed"
		}

		// Format times
		var startTime, completionTime *string
		if job.Status.StartTime != nil {
			st := job.Status.StartTime.Format(time.RFC3339)
			startTime = &st
		}
		if job.Status.CompletionTime != nil {
			ct := job.Status.CompletionTime.Format(time.RFC3339)
			completionTime = &ct
		}

		return c.JSON(http.StatusOK, JobStatus{
			JobName:        jobName,
			Status:         status,
			Active:         job.Status.Active,
			Succeeded:      job.Status.Succeeded,
			Failed:         job.Status.Failed,
			StartTime:      startTime,
			CompletionTime: completionTime,
		})
	}
}
