// Author: Enkae (enkae.dev@pm.me)
package adapter

import (
	"context"
	"database/sql"
	"fmt"
	"sync"

	"ghost/kernel/internal/domain"
)

// StateRepository manages global application state
type StateRepository struct {
	db    *sql.DB
	mu    sync.RWMutex
	cache domain.AppState // In-memory cache for fast reads
}

// NewStateRepository creates a new state repository instance
func NewStateRepository(db *sql.DB) (*StateRepository, error) {
	repo := &StateRepository{
		db:    db,
		cache: domain.AppStateShadow, // Default to safe mode
	}

	// Create table if not exists
	if err := repo.createTable(); err != nil {
		return nil, fmt.Errorf("failed to create state table: %w", err)
	}

	// Initialize with default state if empty
	if err := repo.initializeState(); err != nil {
		return nil, fmt.Errorf("failed to initialize state: %w", err)
	}

	// Load current state into cache
	if err := repo.loadCache(); err != nil {
		return nil, fmt.Errorf("failed to load state cache: %w", err)
	}

	return repo, nil
}

// createTable creates the app_state table
func (r *StateRepository) createTable() error {
	query := `
	CREATE TABLE IF NOT EXISTS app_state (
		id INTEGER PRIMARY KEY CHECK (id = 1),
		state TEXT NOT NULL,
		updated_at DATETIME NOT NULL
	);
	`

	_, err := r.db.Exec(query)
	return err
}

// initializeState sets default state if table is empty
func (r *StateRepository) initializeState() error {
	var count int
	err := r.db.QueryRow("SELECT COUNT(*) FROM app_state").Scan(&count)
	if err != nil {
		return err
	}

	if count == 0 {
		// Insert default SHADOW state
		query := `INSERT INTO app_state (id, state, updated_at) VALUES (1, ?, datetime('now'))`
		_, err = r.db.Exec(query, domain.AppStateShadow)
		return err
	}

	return nil
}

// loadCache loads the current state from database into memory
func (r *StateRepository) loadCache() error {
	var stateStr string
	query := "SELECT state FROM app_state WHERE id = 1"

	err := r.db.QueryRow(query).Scan(&stateStr)
	if err != nil {
		return err
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	r.cache = domain.AppState(stateStr)
	return nil
}

// GetState returns the current application state (fast, cached)
func (r *StateRepository) GetState(ctx context.Context) (domain.AppState, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.cache, nil
}

// SetState updates the application state
func (r *StateRepository) SetState(ctx context.Context, state domain.AppState) error {
	// Validate state
	if !state.IsValid() {
		return fmt.Errorf("invalid app state: %s", state)
	}

	// Update database
	query := `UPDATE app_state SET state = ?, updated_at = datetime('now') WHERE id = 1`
	_, err := r.db.Exec(query, state)
	if err != nil {
		return fmt.Errorf("failed to update state in database: %w", err)
	}

	// Update cache
	r.mu.Lock()
	r.cache = state
	r.mu.Unlock()

	return nil
}
