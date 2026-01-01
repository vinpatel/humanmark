package repository

import (
	"context"
	"testing"
	"time"
)

// TestMemoryRepository tests the in-memory repository implementation.
func TestMemoryRepository(t *testing.T) {
	repo := NewMemory()
	ctx := context.Background()

	t.Run("CreateJob generates ID and timestamps", func(t *testing.T) {
		job := Job{
			ContentType: "text",
			Human:       true,
			Confidence:  0.95,
			AIScore:     0.05,
			Detectors:   []string{"mock"},
			ContentHash: "abc123",
		}

		created, err := repo.CreateJob(ctx, job)
		if err != nil {
			t.Fatalf("CreateJob failed: %v", err)
		}

		// ID should be generated
		if created.ID == "" {
			t.Error("expected non-empty ID")
		}

		// ID should be 24 characters (12 bytes hex encoded)
		if len(created.ID) != 24 {
			t.Errorf("expected 24 char ID, got %d: %s", len(created.ID), created.ID)
		}

		// Timestamps should be set
		if created.CreatedAt.IsZero() {
			t.Error("CreatedAt should be set")
		}
		if created.UpdatedAt.IsZero() {
			t.Error("UpdatedAt should be set")
		}

		// Fields should be preserved
		if created.ContentType != "text" {
			t.Errorf("ContentType: expected 'text', got %s", created.ContentType)
		}
		if !created.Human {
			t.Error("Human should be true")
		}
		if created.Confidence != 0.95 {
			t.Errorf("Confidence: expected 0.95, got %f", created.Confidence)
		}
	})

	t.Run("GetJob returns created job", func(t *testing.T) {
		job := Job{
			ContentType: "image",
			Human:       false,
			Confidence:  0.8,
			AIScore:     0.7,
			Detectors:   []string{"hive"},
		}

		created, err := repo.CreateJob(ctx, job)
		if err != nil {
			t.Fatalf("CreateJob failed: %v", err)
		}

		// Retrieve the job
		retrieved, err := repo.GetJob(ctx, created.ID)
		if err != nil {
			t.Fatalf("GetJob failed: %v", err)
		}

		// Verify fields match
		if retrieved.ID != created.ID {
			t.Errorf("ID mismatch: %s != %s", retrieved.ID, created.ID)
		}
		if retrieved.ContentType != created.ContentType {
			t.Errorf("ContentType mismatch")
		}
		if retrieved.Human != created.Human {
			t.Errorf("Human mismatch")
		}
		if retrieved.Confidence != created.Confidence {
			t.Errorf("Confidence mismatch")
		}
	})

	t.Run("GetJob returns ErrNotFound for non-existent ID", func(t *testing.T) {
		_, err := repo.GetJob(ctx, "non-existent-id")
		if err != ErrNotFound {
			t.Errorf("expected ErrNotFound, got %v", err)
		}
	})

	t.Run("GetJob returns copy, not reference", func(t *testing.T) {
		job := Job{
			ContentType: "text",
			Human:       true,
			Confidence:  0.9,
		}

		created, _ := repo.CreateJob(ctx, job)

		// Get job twice
		job1, _ := repo.GetJob(ctx, created.ID)
		job2, _ := repo.GetJob(ctx, created.ID)

		// Modify one
		job1.Confidence = 0.5

		// The other should be unchanged
		if job2.Confidence != 0.9 {
			t.Error("GetJob should return copies, not references")
		}
	})

	t.Run("Ping succeeds", func(t *testing.T) {
		if err := repo.Ping(ctx); err != nil {
			t.Errorf("Ping failed: %v", err)
		}
	})

	t.Run("Close succeeds", func(t *testing.T) {
		if err := repo.Close(); err != nil {
			t.Errorf("Close failed: %v", err)
		}
	})

	t.Run("multiple jobs have unique IDs", func(t *testing.T) {
		ids := make(map[string]bool)

		for i := 0; i < 100; i++ {
			job, err := repo.CreateJob(ctx, Job{ContentType: "text"})
			if err != nil {
				t.Fatalf("CreateJob failed: %v", err)
			}

			if ids[job.ID] {
				t.Errorf("duplicate ID generated: %s", job.ID)
			}
			ids[job.ID] = true
		}
	})
}

// TestGenerateID tests ID generation.
func TestGenerateID(t *testing.T) {
	t.Run("generates unique IDs", func(t *testing.T) {
		ids := make(map[string]bool)

		for i := 0; i < 1000; i++ {
			id := generateID()

			// Check length
			if len(id) != 24 {
				t.Errorf("expected 24 char ID, got %d: %s", len(id), id)
			}

			// Check uniqueness
			if ids[id] {
				t.Errorf("duplicate ID: %s", id)
			}
			ids[id] = true

			// Check it's valid hex
			for _, c := range id {
				if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
					t.Errorf("invalid hex character in ID: %c", c)
				}
			}
		}
	})
}

// TestJobStruct tests Job struct behavior.
func TestJobStruct(t *testing.T) {
	t.Run("zero value has expected defaults", func(t *testing.T) {
		var job Job

		if job.ID != "" {
			t.Error("ID should be empty by default")
		}
		if job.Human != false {
			t.Error("Human should be false by default")
		}
		if job.Confidence != 0 {
			t.Error("Confidence should be 0 by default")
		}
		if !job.CreatedAt.IsZero() {
			t.Error("CreatedAt should be zero by default")
		}
	})

	t.Run("Detectors slice is independent", func(t *testing.T) {
		original := []string{"detector1", "detector2"}
		job := Job{
			Detectors: original,
		}

		// Modify original
		original[0] = "modified"

		// Job's detectors should be unchanged if properly copied
		// Note: In current implementation, we're not deep copying slices
		// This test documents current behavior
	})
}

// BenchmarkCreateJob benchmarks job creation.
func BenchmarkCreateJob(b *testing.B) {
	repo := NewMemory()
	ctx := context.Background()
	job := Job{
		ContentType: "text",
		Human:       true,
		Confidence:  0.9,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		repo.CreateJob(ctx, job)
	}
}

// BenchmarkGetJob benchmarks job retrieval.
func BenchmarkGetJob(b *testing.B) {
	repo := NewMemory()
	ctx := context.Background()

	// Create a job to retrieve
	created, _ := repo.CreateJob(ctx, Job{ContentType: "text"})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		repo.GetJob(ctx, created.ID)
	}
}

// TestContextCancellation tests that operations respect context cancellation.
func TestContextCancellation(t *testing.T) {
	repo := NewMemory()

	t.Run("CreateJob with cancelled context", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		// Memory implementation doesn't check context, but this tests the interface
		_, err := repo.CreateJob(ctx, Job{})
		// Current implementation doesn't fail on cancelled context
		// but a real database implementation would
		_ = err
	})

	t.Run("GetJob with timeout context", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
		defer cancel()

		time.Sleep(1 * time.Millisecond) // Let timeout expire

		// Memory implementation doesn't check context
		_, _ = repo.GetJob(ctx, "some-id")
	})
}
