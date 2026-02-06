package adapter

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// IntentHistoryEntry represents a successful intent execution
type IntentHistoryEntry struct {
	ID            string    `json:"id"`
	Intent        string    `json:"intent"`
	FocusedWindow string    `json:"focused_window"`
	ExecutedAt    time.Time `json:"executed_at"`
	SuccessCount  int       `json:"success_count"`
	CachedPlan    string    `json:"cached_plan,omitempty"`
}

// IntentHistoryRepository manages intent history for trust scoring
type IntentHistoryRepository struct {
	db *sql.DB
}

// NewIntentHistoryRepository creates a new IntentHistoryRepository and initializes tables
func NewIntentHistoryRepository(db *sql.DB) (*IntentHistoryRepository, error) {
	repo := &IntentHistoryRepository{db: db}

	// Create intent_history table
	createTableSQL := `
	CREATE TABLE IF NOT EXISTS intent_history (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		intent TEXT NOT NULL,
		focused_window TEXT NOT NULL,
		executed_at DATETIME NOT NULL,
		success_count INTEGER DEFAULT 1,
		cached_plan TEXT
	);
	`

	if _, err := db.Exec(createTableSQL); err != nil {
		return nil, fmt.Errorf("failed to create intent_history table: %w", err)
	}

	// Migrate existing tables to add cached_plan column if it doesn't exist
	migrateSQL := "ALTER TABLE intent_history ADD COLUMN cached_plan TEXT;"
	// Ignore error if column already exists
	_, _ = db.Exec(migrateSQL)

	// Create index for fast lookups by intent and window
	createIndexSQL := `
	CREATE INDEX IF NOT EXISTS idx_intent_window ON intent_history(intent, focused_window);
	`

	if _, err := db.Exec(createIndexSQL); err != nil {
		return nil, fmt.Errorf("failed to create intent_history index: %w", err)
	}

	return repo, nil
}

// RecordSuccess records a successful intent execution with optional plan caching
func (r *IntentHistoryRepository) RecordSuccess(ctx context.Context, intent string, focusedWindow string, cachedPlan ...string) error {
	var planJSON string
	if len(cachedPlan) > 0 {
		planJSON = cachedPlan[0]
	}
	// Check if this intent/window combination already exists
	var existingID int
	var successCount int

	querySQL := `
	SELECT id, success_count FROM intent_history
	WHERE intent = ? AND focused_window = ?
	LIMIT 1
	`

	err := r.db.QueryRowContext(ctx, querySQL, intent, focusedWindow).Scan(&existingID, &successCount)

	if err == sql.ErrNoRows {
		// First time this intent/window combo was used - insert new record
		insertSQL := `
		INSERT INTO intent_history (intent, focused_window, executed_at, success_count, cached_plan)
		VALUES (?, ?, ?, 1, ?)
		`

		_, err := r.db.ExecContext(ctx, insertSQL, intent, focusedWindow, time.Now(), planJSON)
		if err != nil {
			return fmt.Errorf("failed to insert intent history: %w", err)
		}
		return nil
	}

	if err != nil {
		return fmt.Errorf("failed to query existing intent history: %w", err)
	}

	// Already exists - increment success count and update timestamp
	// Also update cached_plan if provided
	updateSQL := `
	UPDATE intent_history
	SET success_count = ?, executed_at = ?, cached_plan = ?
	WHERE id = ?
	`

	_, err = r.db.ExecContext(ctx, updateSQL, successCount+1, time.Now(), planJSON, existingID)
	if err != nil {
		return fmt.Errorf("failed to update intent history: %w", err)
	}

	return nil
}

// GetTrustScore retrieves the trust score for an intent/window combination
// Returns the number of successful executions, or 0 if never executed before
func (r *IntentHistoryRepository) GetTrustScore(ctx context.Context, intent string, focusedWindow string) (int, error) {
	querySQL := `
	SELECT success_count FROM intent_history
	WHERE intent = ? AND focused_window = ?
	LIMIT 1
	`

	var successCount int
	err := r.db.QueryRowContext(ctx, querySQL, intent, focusedWindow).Scan(&successCount)

	if err == sql.ErrNoRows {
		// Never executed before - return 0
		return 0, nil
	}

	if err != nil {
		return 0, fmt.Errorf("failed to query trust score: %w", err)
	}

	return successCount, nil
}

// GetReflex retrieves a cached plan for an intent if trust score is high enough
// Returns the cached plan JSON and trust score, or empty string if not found or trust too low
func (r *IntentHistoryRepository) GetReflex(ctx context.Context, intent string) (string, int, error) {
	querySQL := `
	SELECT cached_plan, success_count
	FROM intent_history
	WHERE intent = ? AND success_count > 5 AND cached_plan IS NOT NULL AND cached_plan != ''
	ORDER BY executed_at DESC
	LIMIT 1
	`

	var cachedPlan sql.NullString
	var successCount int

	err := r.db.QueryRowContext(ctx, querySQL, intent).Scan(&cachedPlan, &successCount)

	if err == sql.ErrNoRows {
		// No reflex found - return empty
		return "", 0, nil
	}

	if err != nil {
		return "", 0, fmt.Errorf("failed to query reflex: %w", err)
	}

	if !cachedPlan.Valid {
		return "", successCount, nil
	}

	return cachedPlan.String, successCount, nil
}

// InvalidateReflex removes the cached plan for a specific intent
// Used when a muscle memory plan fails and needs to be re-learned
func (r *IntentHistoryRepository) InvalidateReflex(ctx context.Context, intent string) error {
	updateSQL := `
	UPDATE intent_history
	SET cached_plan = NULL
	WHERE intent = ?
	`

	_, err := r.db.ExecContext(ctx, updateSQL, intent)
	if err != nil {
		return fmt.Errorf("failed to invalidate reflex: %w", err)
	}

	return nil
}

// GetRecentHistory retrieves the most recent N successful intent executions
func (r *IntentHistoryRepository) GetRecentHistory(ctx context.Context, limit int) ([]IntentHistoryEntry, error) {
	querySQL := `
	SELECT id, intent, focused_window, executed_at, success_count
	FROM intent_history
	ORDER BY executed_at DESC
	LIMIT ?
	`

	rows, err := r.db.QueryContext(ctx, querySQL, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query recent history: %w", err)
	}
	defer rows.Close()

	var entries []IntentHistoryEntry

	for rows.Next() {
		var entry IntentHistoryEntry
		var id int

		err := rows.Scan(
			&id,
			&entry.Intent,
			&entry.FocusedWindow,
			&entry.ExecutedAt,
			&entry.SuccessCount,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan history entry: %w", err)
		}

		entry.ID = fmt.Sprintf("%d", id)
		entries = append(entries, entry)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %w", err)
	}

	return entries, nil
}
