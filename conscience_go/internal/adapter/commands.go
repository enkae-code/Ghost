// Author: Enkae (enkae.dev@pm.me)
package adapter

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"ghost/kernel/internal/domain"
)

// CommandRepository manages command persistence and retrieval
type CommandRepository struct {
	db *sql.DB
}

// NewCommandRepository creates a new command repository
func NewCommandRepository(db *sql.DB) (*CommandRepository, error) {
	// Create commands table if not exists
	createTableSQL := `
	CREATE TABLE IF NOT EXISTS commands (
		id TEXT PRIMARY KEY,
		action TEXT NOT NULL,
		target TEXT NOT NULL,
		payload TEXT NOT NULL,
		status TEXT NOT NULL,
		created_at DATETIME NOT NULL,
		executed_at DATETIME
	);
	`

	if _, err := db.Exec(createTableSQL); err != nil {
		return nil, fmt.Errorf("failed to create commands table: %w", err)
	}

	return &CommandRepository{db: db}, nil
}

// SaveCommand persists a command to the database
func (r *CommandRepository) SaveCommand(ctx context.Context, cmd *domain.Command) error {
	insertSQL := `
	INSERT INTO commands (id, action, target, payload, status, created_at)
	VALUES (?, ?, ?, ?, ?, ?)
	`

	_, err := r.db.ExecContext(
		ctx,
		insertSQL,
		cmd.ID,
		string(cmd.Action),
		cmd.Target,
		cmd.Payload,
		string(cmd.Status),
		cmd.CreatedAt,
	)

	if err != nil {
		return fmt.Errorf("failed to insert command: %w", err)
	}

	return nil
}

// GetPendingCommands retrieves all pending commands
func (r *CommandRepository) GetPendingCommands(ctx context.Context) ([]domain.Command, error) {
	query := `
	SELECT id, action, target, payload, status, created_at, executed_at
	FROM commands
	WHERE status = ?
	ORDER BY created_at ASC
	`

	rows, err := r.db.QueryContext(ctx, query, string(domain.CommandStatusPending))
	if err != nil {
		return nil, fmt.Errorf("failed to query pending commands: %w", err)
	}
	defer rows.Close()

	var commands []domain.Command
	for rows.Next() {
		var cmd domain.Command
		var action string
		var status string
		var executedAt sql.NullTime

		err := rows.Scan(
			&cmd.ID,
			&action,
			&cmd.Target,
			&cmd.Payload,
			&status,
			&cmd.CreatedAt,
			&executedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan command: %w", err)
		}

		cmd.Action = domain.CommandAction(action)
		cmd.Status = domain.CommandStatus(status)
		if executedAt.Valid {
			cmd.ExecutedAt = &executedAt.Time
		}

		commands = append(commands, cmd)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating command rows: %w", err)
	}

	return commands, nil
}

// UpdateCommandStatus updates the status of a command
func (r *CommandRepository) UpdateCommandStatus(ctx context.Context, id string, status domain.CommandStatus) error {
	now := time.Now()
	updateSQL := `
	UPDATE commands
	SET status = ?, executed_at = ?
	WHERE id = ?
	`

	result, err := r.db.ExecContext(ctx, updateSQL, string(status), now, id)
	if err != nil {
		return fmt.Errorf("failed to update command status: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("command not found: %s", id)
	}

	return nil
}
