package utils

import (
	"testing"
	"time"

	"grabbi-backend/dtos"

	"github.com/google/uuid"
)

func newTestStore() *JobStore {
	return &JobStore{
		jobs: make(map[uuid.UUID]*dtos.BatchJob),
	}
}

func TestCreateJob(t *testing.T) {
	store := newTestStore()
	job := store.CreateJob(10)

	if job == nil {
		t.Fatal("expected job, got nil")
	}
	if job.Status != dtos.JobStatusPending {
		t.Errorf("expected status %q, got %q", dtos.JobStatusPending, job.Status)
	}
	if job.Progress != 0 {
		t.Errorf("expected progress 0, got %d", job.Progress)
	}
	if job.Total != 10 {
		t.Errorf("expected total 10, got %d", job.Total)
	}
	if job.Processed != 0 {
		t.Errorf("expected processed 0, got %d", job.Processed)
	}
	if job.Created != 0 {
		t.Errorf("expected created 0, got %d", job.Created)
	}
	if job.Updated != 0 {
		t.Errorf("expected updated 0, got %d", job.Updated)
	}
	if job.Deleted != 0 {
		t.Errorf("expected deleted 0, got %d", job.Deleted)
	}
	if job.Failed != 0 {
		t.Errorf("expected failed 0, got %d", job.Failed)
	}
	if job.ID == uuid.Nil {
		t.Error("expected non-nil job ID")
	}
}

func TestGetJobExists(t *testing.T) {
	store := newTestStore()
	job := store.CreateJob(5)

	found, ok := store.GetJob(job.ID)
	if !ok {
		t.Fatal("expected to find job")
	}
	if found.ID != job.ID {
		t.Errorf("expected job ID %s, got %s", job.ID, found.ID)
	}
}

func TestGetJobNotFound(t *testing.T) {
	store := newTestStore()

	_, ok := store.GetJob(uuid.New())
	if ok {
		t.Fatal("expected job not found")
	}
}

func TestUpdateJob(t *testing.T) {
	store := newTestStore()
	job := store.CreateJob(10)

	store.UpdateJob(job.ID, func(j *dtos.BatchJob) {
		j.Processed = 5
		j.Progress = 50
	})

	updated, ok := store.GetJob(job.ID)
	if !ok {
		t.Fatal("expected to find job")
	}
	if updated.Processed != 5 {
		t.Errorf("expected processed 5, got %d", updated.Processed)
	}
	if updated.Progress != 50 {
		t.Errorf("expected progress 50, got %d", updated.Progress)
	}
}

func TestCompleteJob(t *testing.T) {
	store := newTestStore()
	job := store.CreateJob(10)

	store.CompleteJob(job.ID, dtos.JobStatusCompleted)

	completed, ok := store.GetJob(job.ID)
	if !ok {
		t.Fatal("expected to find job")
	}
	if completed.Status != dtos.JobStatusCompleted {
		t.Errorf("expected status %q, got %q", dtos.JobStatusCompleted, completed.Status)
	}
	if completed.Progress != 100 {
		t.Errorf("expected progress 100, got %d", completed.Progress)
	}
	if completed.CompletedAt == nil {
		t.Fatal("expected CompletedAt to be set")
	}
}

func TestAddCreated(t *testing.T) {
	store := newTestStore()
	job := store.CreateJob(5)

	store.AddCreated(job.ID)
	store.AddCreated(job.ID)

	found, _ := store.GetJob(job.ID)
	if found.Created != 2 {
		t.Errorf("expected created 2, got %d", found.Created)
	}
}

func TestAddUpdated(t *testing.T) {
	store := newTestStore()
	job := store.CreateJob(5)

	store.AddUpdated(job.ID)
	store.AddUpdated(job.ID)
	store.AddUpdated(job.ID)

	found, _ := store.GetJob(job.ID)
	if found.Updated != 3 {
		t.Errorf("expected updated 3, got %d", found.Updated)
	}
}

func TestAddDeleted(t *testing.T) {
	store := newTestStore()
	job := store.CreateJob(5)

	store.AddDeleted(job.ID)

	found, _ := store.GetJob(job.ID)
	if found.Deleted != 1 {
		t.Errorf("expected deleted 1, got %d", found.Deleted)
	}
}

func TestSetProcessing(t *testing.T) {
	store := newTestStore()
	job := store.CreateJob(5)

	store.SetProcessing(job.ID)

	found, _ := store.GetJob(job.ID)
	if found.Status != dtos.JobStatusProcessing {
		t.Errorf("expected status %q, got %q", dtos.JobStatusProcessing, found.Status)
	}
}

func TestCleanupOldJobs(t *testing.T) {
	store := newTestStore()
	job := store.CreateJob(5)

	// Manually set CompletedAt to 2 hours ago so it qualifies for cleanup
	twoHoursAgo := time.Now().Add(-2 * time.Hour)
	store.UpdateJob(job.ID, func(j *dtos.BatchJob) {
		j.Status = dtos.JobStatusCompleted
		j.CompletedAt = &twoHoursAgo
	})

	store.CleanupOldJobs()

	_, ok := store.GetJob(job.ID)
	if ok {
		t.Fatal("expected old completed job to be cleaned up")
	}
}

func TestCleanupKeepsRecentJobs(t *testing.T) {
	store := newTestStore()
	job := store.CreateJob(5)

	// Complete the job just now - should NOT be cleaned up
	store.CompleteJob(job.ID, dtos.JobStatusCompleted)

	store.CleanupOldJobs()

	_, ok := store.GetJob(job.ID)
	if !ok {
		t.Fatal("expected recent completed job to be kept")
	}
}
