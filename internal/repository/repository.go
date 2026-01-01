// Package repository provides data persistence for HumanMark.
//
// The Repository interface abstracts storage operations, allowing different
// implementations for different environments:
//   - MemoryRepository: In-memory storage for development/testing
//   - PostgresRepository: PostgreSQL for production
//
// This follows the repository pattern, separating data access from business logic.
package repository

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"sync"
	"time"
)

// Common errors
var (
	ErrNotFound = errors.New("not found")
)

// Job represents a verification job in the database.
type Job struct {
	// ID is the unique identifier (public-facing)
	ID string

	// ContentType is what type of content was analyzed
	ContentType string

	// Human is true if content was created by a human
	Human bool

	// Confidence is how confident we are (0.0-1.0)
	Confidence float64

	// AIScore is the raw AI probability (0.0-1.0)
	AIScore float64

	// Detectors lists which detection methods were used
	Detectors []string

	// ContentHash is SHA256 hash of the analyzed content
	ContentHash string

	// CreatedAt is when the job was created
	CreatedAt time.Time

	// UpdatedAt is when the job was last updated
	UpdatedAt time.Time
}

// Repository defines the interface for job persistence.
type Repository interface {
	// CreateJob creates a new job and returns it with generated ID.
	CreateJob(ctx context.Context, job Job) (*Job, error)

	// GetJob retrieves a job by ID.
	GetJob(ctx context.Context, id string) (*Job, error)

	// Ping checks database connectivity.
	Ping(ctx context.Context) error

	// Close releases database resources.
	Close() error
}

// =============================================================================
// In-Memory Implementation (for development/testing)
// =============================================================================

// memoryRepository implements Repository using in-memory storage.
// This is useful for development and testing when no database is available.
type memoryRepository struct {
	mu   sync.RWMutex
	jobs map[string]*Job
}

// NewMemory creates a new in-memory repository.
func NewMemory() Repository {
	return &memoryRepository{
		jobs: make(map[string]*Job),
	}
}

// CreateJob creates a new job in memory.
func (r *memoryRepository) CreateJob(ctx context.Context, job Job) (*Job, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Generate ID
	job.ID = generateID()
	job.CreatedAt = time.Now()
	job.UpdatedAt = job.CreatedAt

	// Store copy
	stored := job
	r.jobs[job.ID] = &stored

	return &job, nil
}

// GetJob retrieves a job from memory.
func (r *memoryRepository) GetJob(ctx context.Context, id string) (*Job, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if job, ok := r.jobs[id]; ok {
		// Return copy
		result := *job
		return &result, nil
	}

	return nil, ErrNotFound
}

// Ping always succeeds for in-memory repository.
func (r *memoryRepository) Ping(ctx context.Context) error {
	return nil
}

// Close is a no-op for in-memory repository.
func (r *memoryRepository) Close() error {
	return nil
}

// =============================================================================
// PostgreSQL Implementation
// =============================================================================

// Note: This is a placeholder. In production, use a real PostgreSQL driver
// like pgx or database/sql with lib/pq.

// postgresRepository implements Repository using PostgreSQL.
type postgresRepository struct {
	connString string
	// db *pgxpool.Pool // Uncomment when using pgx
}

// NewPostgres creates a new PostgreSQL repository.
func NewPostgres(connString string) (Repository, error) {
	if connString == "" {
		return nil, errors.New("database connection string is required")
	}

	// TODO: Initialize actual database connection
	// pool, err := pgxpool.New(context.Background(), connString)
	// if err != nil {
	//     return nil, fmt.Errorf("failed to connect to database: %w", err)
	// }

	return &postgresRepository{
		connString: connString,
	}, nil
}

// CreateJob creates a new job in PostgreSQL.
func (r *postgresRepository) CreateJob(ctx context.Context, job Job) (*Job, error) {
	job.ID = generateID()
	job.CreatedAt = time.Now()
	job.UpdatedAt = job.CreatedAt

	// TODO: Actual database insert
	// _, err := r.db.Exec(ctx,
	//     `INSERT INTO jobs (id, content_type, human, confidence, ai_score, detectors, content_hash, created_at, updated_at)
	//      VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
	//     job.ID, job.ContentType, job.Human, job.Confidence, job.AIScore,
	//     job.Detectors, job.ContentHash, job.CreatedAt, job.UpdatedAt,
	// )

	return &job, nil
}

// GetJob retrieves a job from PostgreSQL.
func (r *postgresRepository) GetJob(ctx context.Context, id string) (*Job, error) {
	// TODO: Actual database query
	// row := r.db.QueryRow(ctx,
	//     `SELECT id, content_type, human, confidence, ai_score, detectors, content_hash, created_at, updated_at
	//      FROM jobs WHERE id = $1`, id,
	// )
	// var job Job
	// err := row.Scan(&job.ID, &job.ContentType, &job.Human, &job.Confidence,
	//     &job.AIScore, &job.Detectors, &job.ContentHash, &job.CreatedAt, &job.UpdatedAt)
	// if err == pgx.ErrNoRows {
	//     return nil, ErrNotFound
	// }

	return nil, ErrNotFound
}

// Ping checks PostgreSQL connectivity.
func (r *postgresRepository) Ping(ctx context.Context) error {
	// TODO: Actual ping
	// return r.db.Ping(ctx)
	return nil
}

// Close releases PostgreSQL connection pool.
func (r *postgresRepository) Close() error {
	// TODO: Actual close
	// r.db.Close()
	return nil
}

// =============================================================================
// Helpers
// =============================================================================

// generateID creates a random URL-safe ID.
func generateID() string {
	bytes := make([]byte, 12)
	rand.Read(bytes)
	return hex.EncodeToString(bytes)
}
