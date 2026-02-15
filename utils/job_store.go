package utils

import (
	"sync"
	"time"

	"grabbi-backend/dtos"

	"github.com/google/uuid"
)

// JobStore manages batch jobs in memory
type JobStore struct {
	jobs map[uuid.UUID]*dtos.BatchJob
	mu   sync.RWMutex
}

// Global job store instance
var Store = &JobStore{
	jobs: make(map[uuid.UUID]*dtos.BatchJob),
}

// CleanupOldJobs removes completed/failed jobs older than 1 hour.
func (js *JobStore) CleanupOldJobs() {
	js.mu.Lock()
	defer js.mu.Unlock()

	cutoff := time.Now().Add(-1 * time.Hour)
	for id, job := range js.jobs {
		if job.CompletedAt != nil && job.CompletedAt.Before(cutoff) {
			delete(js.jobs, id)
		} else if job.StartedAt.Before(cutoff) && (job.Status == dtos.JobStatusCompleted || job.Status == dtos.JobStatusFailed) {
			delete(js.jobs, id)
		}
	}
}

// CreateJob creates a new batch job
func (js *JobStore) CreateJob(totalProducts int) *dtos.BatchJob {
	// Clean up old jobs on each new creation
	js.CleanupOldJobs()

	js.mu.Lock()
	defer js.mu.Unlock()

	job := &dtos.BatchJob{
		ID:        uuid.New(),
		Status:    dtos.JobStatusPending,
		Progress:  0,
		Total:     totalProducts,
		Processed: 0,
		Created:   0,
		Updated:   0,
		Deleted:   0,
		Failed:    0,
		Errors:    []dtos.JobError{},
		StartedAt: time.Now(),
	}

	js.jobs[job.ID] = job
	return job
}

// GetJob retrieves a job by ID
func (js *JobStore) GetJob(id uuid.UUID) (*dtos.BatchJob, bool) {
	js.mu.RLock()
	defer js.mu.RUnlock()

	job, exists := js.jobs[id]
	return job, exists
}

// UpdateJob updates job status and progress
func (js *JobStore) UpdateJob(id uuid.UUID, updates func(*dtos.BatchJob)) {
	js.mu.Lock()
	defer js.mu.Unlock()

	if job, exists := js.jobs[id]; exists {
		updates(job)
	}
}

// CompleteJob marks a job as completed
func (js *JobStore) CompleteJob(id uuid.UUID, status string) {
	js.mu.Lock()
	defer js.mu.Unlock()

	if job, exists := js.jobs[id]; exists {
		job.Status = status
		job.Progress = 100
		now := time.Now()
		job.CompletedAt = &now
	}
}

// AddCreated increments created counter
func (js *JobStore) AddCreated(id uuid.UUID) {
	js.mu.Lock()
	defer js.mu.Unlock()

	if job, exists := js.jobs[id]; exists {
		job.Created++
	}
}

// AddUpdated increments updated counter
func (js *JobStore) AddUpdated(id uuid.UUID) {
	js.mu.Lock()
	defer js.mu.Unlock()

	if job, exists := js.jobs[id]; exists {
		job.Updated++
	}
}

// AddDeleted increments deleted counter
func (js *JobStore) AddDeleted(id uuid.UUID) {
	js.mu.Lock()
	defer js.mu.Unlock()

	if job, exists := js.jobs[id]; exists {
		job.Deleted++
	}
}

// SetProcessing marks job as processing
func (js *JobStore) SetProcessing(id uuid.UUID) {
	js.mu.Lock()
	defer js.mu.Unlock()

	if job, exists := js.jobs[id]; exists {
		job.Status = dtos.JobStatusProcessing
	}
}
