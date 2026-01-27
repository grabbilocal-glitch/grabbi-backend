package dtos

import (
	"time"

	"github.com/google/uuid"
)

// BatchJob represents a batch import/export job status
type BatchJob struct {
	ID          uuid.UUID  `gorm:"type:uuid;primary_key" json:"id"`
	Status      string     `json:"status"`   // pending, processing, completed, failed
	Progress    int        `json:"progress"` // 0-100 percentage
	Total       int        `json:"total"`
	Processed   int        `json:"processed"`
	Created     int        `json:"created"`
	Updated     int        `json:"updated"`
	Deleted     int        `json:"deleted"`
	Failed      int        `json:"failed"`
	Errors      []JobError `json:"errors"`
	StartedAt   time.Time  `json:"started_at"`
	CompletedAt *time.Time `json:"completed_at"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

// JobError represents a single error in a batch job
type JobError struct {
	Row     int               `json:"row"`     // Row number in Excel
	Product string            `json:"product"` // Product name
	Fields  map[string]string `json:"fields"`  // Field -> Error message
}

// JobStatus constants
const (
	JobStatusPending    = "pending"
	JobStatusProcessing = "processing"
	JobStatusCompleted  = "completed"
	JobStatusFailed     = "failed"
)
