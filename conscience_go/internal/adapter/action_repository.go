package adapter

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"ghost/kernel/internal/domain"
)

// ActionRepository manages action proposal persistence and user mode settings
type ActionRepository struct {
	db *sql.DB
}

// NewActionRepository creates a new ActionRepository and initializes tables
func NewActionRepository(db *sql.DB) (*ActionRepository, error) {
	repo := &ActionRepository{db: db}

	// Create action_proposals table
	createActionsTableSQL := `
	CREATE TABLE IF NOT EXISTS action_proposals (
		id TEXT PRIMARY KEY,
		intent TEXT NOT NULL,
		risk_score INTEGER NOT NULL,
		status TEXT NOT NULL,
		payload TEXT NOT NULL,
		domain TEXT NOT NULL,
		created_at DATETIME NOT NULL,
		updated_at DATETIME NOT NULL,
		approved_at DATETIME,
		interaction_type TEXT NOT NULL DEFAULT 'PERMISSION',
		agent_message TEXT,
		user_response TEXT
	);
	`

	if _, err := db.Exec(createActionsTableSQL); err != nil {
		return nil, fmt.Errorf("failed to create action_proposals table: %w", err)
	}

	// Migrate existing tables to add new columns if they don't exist
	migrateActionsSQL := []string{
		"ALTER TABLE action_proposals ADD COLUMN interaction_type TEXT NOT NULL DEFAULT 'PERMISSION';",
		"ALTER TABLE action_proposals ADD COLUMN agent_message TEXT;",
		"ALTER TABLE action_proposals ADD COLUMN user_response TEXT;",
	}

	for _, stmt := range migrateActionsSQL {
		// Ignore errors if columns already exist
		_, _ = db.Exec(stmt)
	}

	// Create user_modes table
	createModesTableSQL := `
	CREATE TABLE IF NOT EXISTS user_modes (
		domain TEXT PRIMARY KEY,
		mode TEXT NOT NULL,
		updated_at DATETIME NOT NULL
	);
	`

	if _, err := db.Exec(createModesTableSQL); err != nil {
		return nil, fmt.Errorf("failed to create user_modes table: %w", err)
	}

	// Insert default mode (AUTO for all domains)
	defaultModeSQL := `
	INSERT OR IGNORE INTO user_modes (domain, mode, updated_at)
	VALUES ('*', 'AUTO', ?);
	`

	if _, err := db.Exec(defaultModeSQL, time.Now()); err != nil {
		return nil, fmt.Errorf("failed to insert default mode: %w", err)
	}

	return repo, nil
}

// SaveActionProposal persists an action proposal to the database
func (r *ActionRepository) SaveActionProposal(ctx context.Context, action *domain.ActionProposal) error {
	insertSQL := `
	INSERT INTO action_proposals (id, intent, risk_score, status, payload, domain, created_at, updated_at, approved_at, interaction_type, agent_message, user_response)
	VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	payloadJSON, err := json.Marshal(action.Payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	_, err = r.db.ExecContext(
		ctx,
		insertSQL,
		action.ID,
		action.Intent,
		action.RiskScore,
		string(action.Status),
		string(payloadJSON),
		action.Domain,
		action.CreatedAt,
		action.UpdatedAt,
		action.ApprovedAt,
		string(action.InteractionType),
		action.AgentMessage,
		action.UserResponse,
	)

	if err != nil {
		return fmt.Errorf("failed to insert action proposal: %w", err)
	}

	return nil
}

// UpdateActionStatus updates the status of an action proposal
func (r *ActionRepository) UpdateActionStatus(ctx context.Context, id string, status domain.ActionProposalStatus) error {
	now := time.Now()
	var approvedAt *time.Time

	if status == domain.ActionProposalStatusApproved {
		approvedAt = &now
	}

	updateSQL := `
	UPDATE action_proposals
	SET status = ?, updated_at = ?, approved_at = ?
	WHERE id = ?
	`

	result, err := r.db.ExecContext(ctx, updateSQL, string(status), now, approvedAt, id)
	if err != nil {
		return fmt.Errorf("failed to update action status: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("action proposal not found: %s", id)
	}

	return nil
}

// UpdateUserResponse updates the user's response for an action proposal
// Used for clarifications where the agent needs context from the user
func (r *ActionRepository) UpdateUserResponse(ctx context.Context, id string, userResponse string) error {
	now := time.Now()

	updateSQL := `
	UPDATE action_proposals
	SET user_response = ?, updated_at = ?
	WHERE id = ?
	`

	result, err := r.db.ExecContext(ctx, updateSQL, userResponse, now, id)
	if err != nil {
		return fmt.Errorf("failed to update user response: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("action proposal not found: %s", id)
	}

	return nil
}

// GetActionProposal retrieves a single action proposal by ID
// Alias for GetActionByID for backwards compatibility
func (r *ActionRepository) GetActionProposal(ctx context.Context, id string) (*domain.ActionProposal, error) {
	return r.GetActionByID(ctx, id)
}

// GetActionByID retrieves a single action proposal by ID with full fields
func (r *ActionRepository) GetActionByID(ctx context.Context, id string) (*domain.ActionProposal, error) {
	query := `
	SELECT id, intent, risk_score, status, payload, domain, created_at, updated_at, approved_at, interaction_type, agent_message, user_response
	FROM action_proposals
	WHERE id = ?
	`

	var action domain.ActionProposal
	var payloadJSON string
	var status string
	var interactionType string
	var approvedAt sql.NullTime
	var agentMessage sql.NullString
	var userResponse sql.NullString

	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&action.ID,
		&action.Intent,
		&action.RiskScore,
		&status,
		&payloadJSON,
		&action.Domain,
		&action.CreatedAt,
		&action.UpdatedAt,
		&approvedAt,
		&interactionType,
		&agentMessage,
		&userResponse,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("action proposal not found: %s", id)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to query action proposal: %w", err)
	}

	action.Status = domain.ActionProposalStatus(status)
	action.Payload = json.RawMessage(payloadJSON)
	action.InteractionType = domain.InteractionType(interactionType)

	if approvedAt.Valid {
		action.ApprovedAt = &approvedAt.Time
	}
	if agentMessage.Valid {
		action.AgentMessage = agentMessage.String
	}
	if userResponse.Valid {
		action.UserResponse = userResponse.String
	}

	return &action, nil
}

// GetPendingApprovals retrieves all action proposals waiting for user interaction
// Includes both permission requests and clarification requests
func (r *ActionRepository) GetPendingApprovals(ctx context.Context) ([]*domain.ActionProposal, error) {
	query := `
	SELECT id, intent, risk_score, status, payload, domain, created_at, updated_at, approved_at, interaction_type, agent_message, user_response
	FROM action_proposals
	WHERE status IN (?, ?)
	ORDER BY created_at ASC
	`

	rows, err := r.db.QueryContext(ctx, query,
		string(domain.ActionProposalStatusWaitingForUser),
		string(domain.ActionProposalStatusWaitingForContext))
	if err != nil {
		return nil, fmt.Errorf("failed to query pending approvals: %w", err)
	}
	defer rows.Close()

	var actions []*domain.ActionProposal

	for rows.Next() {
		var action domain.ActionProposal
		var payloadJSON string
		var status string
		var interactionType string
		var approvedAt sql.NullTime
		var agentMessage sql.NullString
		var userResponse sql.NullString

		err := rows.Scan(
			&action.ID,
			&action.Intent,
			&action.RiskScore,
			&status,
			&payloadJSON,
			&action.Domain,
			&action.CreatedAt,
			&action.UpdatedAt,
			&approvedAt,
			&interactionType,
			&agentMessage,
			&userResponse,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan action proposal: %w", err)
		}

		action.Status = domain.ActionProposalStatus(status)
		action.Payload = json.RawMessage(payloadJSON)
		action.InteractionType = domain.InteractionType(interactionType)

		if approvedAt.Valid {
			action.ApprovedAt = &approvedAt.Time
		}
		if agentMessage.Valid {
			action.AgentMessage = agentMessage.String
		}
		if userResponse.Valid {
			action.UserResponse = userResponse.String
		}

		actions = append(actions, &action)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %w", err)
	}

	return actions, nil
}

// GetUserMode retrieves the user mode for a specific domain
func (r *ActionRepository) GetUserMode(ctx context.Context, domainName string) (*domain.UserMode, error) {
	query := `
	SELECT domain, mode, updated_at
	FROM user_modes
	WHERE domain = ? OR domain = '*'
	ORDER BY CASE WHEN domain = ? THEN 0 ELSE 1 END
	LIMIT 1
	`

	var mode domain.UserMode
	var modeType string

	err := r.db.QueryRowContext(ctx, query, domainName, domainName).Scan(
		&mode.Domain,
		&modeType,
		&mode.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		// Return default AUTO mode if not found
		return &domain.UserMode{
			Domain:    "*",
			Mode:      domain.ModeTypeAuto,
			UpdatedAt: time.Now(),
		}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to query user mode: %w", err)
	}

	mode.Mode = domain.ModeType(modeType)

	return &mode, nil
}

// SetUserMode sets the automation mode for a specific domain
func (r *ActionRepository) SetUserMode(ctx context.Context, domainName string, modeType domain.ModeType) error {
	insertSQL := `
	INSERT INTO user_modes (domain, mode, updated_at)
	VALUES (?, ?, ?)
	ON CONFLICT(domain) DO UPDATE SET mode = ?, updated_at = ?
	`

	now := time.Now()

	_, err := r.db.ExecContext(ctx, insertSQL, domainName, string(modeType), now, string(modeType), now)
	if err != nil {
		return fmt.Errorf("failed to set user mode: %w", err)
	}

	return nil
}

// GetApprovedActions retrieves all approved actions ready for execution
// This is the Effector Queue - only APPROVED actions flow to the Sentinel
func (r *ActionRepository) GetApprovedActions(ctx context.Context) ([]*domain.ActionProposal, error) {
	query := `
	SELECT id, intent, risk_score, status, payload, domain, created_at, updated_at, approved_at
	FROM action_proposals
	WHERE status IN (?, ?)
	ORDER BY approved_at ASC, created_at ASC
	`

	rows, err := r.db.QueryContext(
		ctx,
		query,
		string(domain.ActionProposalStatusApproved),
		string(domain.ActionProposalStatusExecuting),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to query approved actions: %w", err)
	}
	defer rows.Close()

	var actions []*domain.ActionProposal

	for rows.Next() {
		var action domain.ActionProposal
		var payloadJSON string
		var status string
		var approvedAt sql.NullTime

		err := rows.Scan(
			&action.ID,
			&action.Intent,
			&action.RiskScore,
			&status,
			&payloadJSON,
			&action.Domain,
			&action.CreatedAt,
			&action.UpdatedAt,
			&approvedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan action proposal: %w", err)
		}

		action.Status = domain.ActionProposalStatus(status)
		action.Payload = json.RawMessage(payloadJSON)

		if approvedAt.Valid {
			action.ApprovedAt = &approvedAt.Time
		}

		actions = append(actions, &action)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %w", err)
	}

	return actions, nil
}
