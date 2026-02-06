// Author: Enkae (enkae.dev@pm.me)
package adapter

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"ghost/kernel/internal/domain"
)

// GoalRepository manages goal persistence for the Agentic Planner
type GoalRepository struct {
	db *sql.DB
}

// NewGoalRepository creates a new GoalRepository and initializes tables
func NewGoalRepository(db *sql.DB) (*GoalRepository, error) {
	repo := &GoalRepository{db: db}

	// Create active_goals table
	createTableSQL := `
	CREATE TABLE IF NOT EXISTS active_goals (
		id TEXT PRIMARY KEY,
		goal_text TEXT NOT NULL,
		status TEXT NOT NULL,
		created_at DATETIME NOT NULL,
		updated_at DATETIME NOT NULL
	);
	`

	if _, err := db.Exec(createTableSQL); err != nil {
		return nil, fmt.Errorf("failed to create active_goals table: %w", err)
	}

	return repo, nil
}

// SaveGoal persists a goal to the database
func (r *GoalRepository) SaveGoal(ctx context.Context, goal *domain.Goal) error {
	insertSQL := `
	INSERT INTO active_goals (id, goal_text, status, created_at, updated_at)
	VALUES (?, ?, ?, ?, ?)
	`

	_, err := r.db.ExecContext(
		ctx,
		insertSQL,
		goal.ID,
		goal.GoalText,
		string(goal.Status),
		goal.CreatedAt,
		goal.UpdatedAt,
	)

	if err != nil {
		return fmt.Errorf("failed to insert goal: %w", err)
	}

	return nil
}

// GetActiveGoal retrieves the first active goal ready for planning
func (r *GoalRepository) GetActiveGoal(ctx context.Context) (*domain.Goal, error) {
	query := `
	SELECT id, goal_text, status, created_at, updated_at
	FROM active_goals
	WHERE status = ?
	ORDER BY created_at ASC
	LIMIT 1
	`

	var goal domain.Goal
	var status string

	err := r.db.QueryRowContext(ctx, query, string(domain.GoalStatusActive)).Scan(
		&goal.ID,
		&goal.GoalText,
		&status,
		&goal.CreatedAt,
		&goal.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, nil // No active goal found (not an error)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to query active goal: %w", err)
	}

	goal.Status = domain.GoalStatus(status)

	return &goal, nil
}

// UpdateGoalStatus updates the status of a goal
func (r *GoalRepository) UpdateGoalStatus(ctx context.Context, id string, status domain.GoalStatus) error {
	now := time.Now()

	updateSQL := `
	UPDATE active_goals
	SET status = ?, updated_at = ?
	WHERE id = ?
	`

	result, err := r.db.ExecContext(ctx, updateSQL, string(status), now, id)
	if err != nil {
		return fmt.Errorf("failed to update goal status: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("goal not found: %s", id)
	}

	return nil
}

// DeleteGoal removes a goal from the database
func (r *GoalRepository) DeleteGoal(ctx context.Context, id string) error {
	deleteSQL := `DELETE FROM active_goals WHERE id = ?`

	result, err := r.db.ExecContext(ctx, deleteSQL, id)
	if err != nil {
		return fmt.Errorf("failed to delete goal: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("goal not found: %s", id)
	}

	return nil
}
