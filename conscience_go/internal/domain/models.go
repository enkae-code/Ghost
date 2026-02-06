// Author: Enkae (enkae.dev@pm.me)
package domain

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// Artifact represents a UI Element captured from the Accessibility Tree
type Artifact struct {
	ID             string       `json:"id"`
	Type           ArtifactType `json:"type"`
	Content        string       `json:"content"`
	BoundingBox    BoundingBox  `json:"bounding_rectangle"`
	Timestamp      time.Time    `json:"timestamp"`
	Classification string       `json:"classification,omitempty"`
	Summary        string       `json:"summary,omitempty"`
}

// ArtifactType defines the type of UI element
type ArtifactType string

const (
	ArtifactTypeWindow   ArtifactType = "window"
	ArtifactTypeButton   ArtifactType = "button"
	ArtifactTypeText     ArtifactType = "text"
	ArtifactTypeEdit     ArtifactType = "edit"
	ArtifactTypeList     ArtifactType = "list"
	ArtifactTypeMenuItem ArtifactType = "menu_item"
	ArtifactTypeUnknown  ArtifactType = "unknown"
)

// BoundingBox represents the geometric bounds of a UI element
type BoundingBox struct {
	Left   int `json:"left"`
	Top    int `json:"top"`
	Right  int `json:"right"`
	Bottom int `json:"bottom"`
}

// ContextFrame represents a snapshot of reality at a specific moment
type ContextFrame struct {
	Timestamp      time.Time  `json:"timestamp"`
	ActiveWindowID string     `json:"active_window_id"`
	Artifacts      []Artifact `json:"artifacts"`
}

// NewContextFrame creates a new context frame with the current timestamp
func NewContextFrame(activeWindowID string) *ContextFrame {
	return &ContextFrame{
		Timestamp:      time.Now(),
		ActiveWindowID: activeWindowID,
		Artifacts:      make([]Artifact, 0),
	}
}

// AddArtifact adds an artifact to the context frame
func (cf *ContextFrame) AddArtifact(artifact Artifact) {
	cf.Artifacts = append(cf.Artifacts, artifact)
}

// Intent represents an action the user wants to take
type Intent struct {
	ID        string          `json:"id"`
	RawPrompt string          `json:"raw_prompt"`
	Plan      json.RawMessage `json:"plan"`
	Status    IntentStatus    `json:"status"`
	CreatedAt time.Time       `json:"created_at"`
	UpdatedAt time.Time       `json:"updated_at"`
}

// IntentStatus represents the current status of an intent
type IntentStatus string

const (
	IntentStatusPending   IntentStatus = "pending"
	IntentStatusApproved  IntentStatus = "approved"
	IntentStatusExecuting IntentStatus = "executing"
	IntentStatusCompleted IntentStatus = "completed"
	IntentStatusFailed    IntentStatus = "failed"
)

// NewIntent creates a new intent with a generated UUID and current timestamp
func NewIntent(rawPrompt string) *Intent {
	now := time.Now()
	return &Intent{
		ID:        uuid.New().String(),
		RawPrompt: rawPrompt,
		Status:    IntentStatusPending,
		CreatedAt: now,
		UpdatedAt: now,
	}
}

// NewArtifact creates a new artifact with a generated UUID and current timestamp
func NewArtifact(artifactType ArtifactType, content string, boundingBox BoundingBox) Artifact {
	return Artifact{
		ID:          uuid.New().String(),
		Type:        artifactType,
		Content:     content,
		BoundingBox: boundingBox,
		Timestamp:   time.Now(),
	}
}

// Command represents an action to be executed by the Sentinel
type Command struct {
	ID        string        `json:"id"`
	Action    CommandAction `json:"action"`
	Target    string        `json:"target"`
	Payload   string        `json:"payload"`
	Status    CommandStatus `json:"status"`
	CreatedAt time.Time     `json:"created_at"`
	ExecutedAt *time.Time   `json:"executed_at,omitempty"`
}

// CommandAction defines the type of action to execute
type CommandAction string

const (
	CommandActionType        CommandAction = "TYPE"
	CommandActionClick       CommandAction = "CLICK"
	CommandActionFocusWindow CommandAction = "FOCUS_WINDOW"
	CommandActionOpenApp     CommandAction = "OPEN_APP"
)

// CommandStatus represents the execution status of a command
type CommandStatus string

const (
	CommandStatusPending   CommandStatus = "pending"
	CommandStatusExecuting CommandStatus = "executing"
	CommandStatusCompleted CommandStatus = "completed"
	CommandStatusFailed    CommandStatus = "failed"
)

// NewCommand creates a new command with a generated UUID and current timestamp
func NewCommand(action CommandAction, target string, payload string) *Command {
	return &Command{
		ID:        uuid.New().String(),
		Action:    action,
		Target:    target,
		Payload:   payload,
		Status:    CommandStatusPending,
		CreatedAt: time.Now(),
	}
}

// ActionProposal represents a proposed action requiring permission
// This is the core of the Permission Kernel - ALL actions flow through this
type ActionProposal struct {
	ID              string              `json:"id"`
	Intent          string              `json:"intent"`
	RiskScore       int                 `json:"risk_score"`
	Status          ActionProposalStatus `json:"status"`
	Payload         json.RawMessage     `json:"payload"`
	Domain          string              `json:"domain"`
	CreatedAt       time.Time           `json:"created_at"`
	UpdatedAt       time.Time           `json:"updated_at"`
	ApprovedAt      *time.Time          `json:"approved_at,omitempty"`

	// Ghost Chat fields for bidirectional communication
	InteractionType InteractionType `json:"interaction_type"`
	AgentMessage    string          `json:"agent_message,omitempty"`
	UserResponse    string          `json:"user_response,omitempty"`
}

// ActionProposalStatus represents the approval state of an action
type ActionProposalStatus string

const (
	ActionProposalStatusPending          ActionProposalStatus = "PENDING"
	ActionProposalStatusWaitingForUser   ActionProposalStatus = "WAITING_FOR_USER"
	ActionProposalStatusWaitingForContext ActionProposalStatus = "WAITING_FOR_CONTEXT"
	ActionProposalStatusApproved         ActionProposalStatus = "APPROVED"
	ActionProposalStatusRejected         ActionProposalStatus = "REJECTED"
	ActionProposalStatusExecuting        ActionProposalStatus = "EXECUTING"
	ActionProposalStatusCompleted        ActionProposalStatus = "COMPLETED"
	ActionProposalStatusFailed           ActionProposalStatus = "FAILED"
)

// InteractionType defines the type of user interaction required
type InteractionType string

const (
	InteractionTypePermission    InteractionType = "PERMISSION"    // Yes/No approval decision
	InteractionTypeClarification InteractionType = "CLARIFICATION" // Open-ended context gathering
)

// UserMode defines automation behavior per application domain
type UserMode struct {
	Domain    string   `json:"domain"`
	Mode      ModeType `json:"mode"`
	UpdatedAt time.Time `json:"updated_at"`
}

// ModeType defines the level of automation
type ModeType string

const (
	ModeTypeAuto   ModeType = "AUTO"   // Auto-approve low-risk actions
	ModeTypeManual ModeType = "MANUAL" // Require explicit approval for all actions
)

// NewActionProposal creates a new action proposal with a generated UUID
func NewActionProposal(intent string, riskScore int, payload json.RawMessage, domain string) *ActionProposal {
	now := time.Now()

	// Determine initial status based on risk score
	status := ActionProposalStatusPending

	return &ActionProposal{
		ID:              uuid.New().String(),
		Intent:          intent,
		RiskScore:       riskScore,
		Status:          status,
		Payload:         payload,
		Domain:          domain,
		CreatedAt:       now,
		UpdatedAt:       now,
		InteractionType: InteractionTypePermission, // Default to permission request
	}
}

// NewClarificationRequest creates an action proposal specifically for clarification
func NewClarificationRequest(intent string, agentMessage string, payload json.RawMessage, domain string) *ActionProposal {
	now := time.Now()

	return &ActionProposal{
		ID:              uuid.New().String(),
		Intent:          intent,
		RiskScore:       0, // Clarifications have no risk
		Status:          ActionProposalStatusWaitingForContext,
		Payload:         payload,
		Domain:          domain,
		CreatedAt:       now,
		UpdatedAt:       now,
		InteractionType: InteractionTypeClarification,
		AgentMessage:    agentMessage,
	}
}

// ShouldAutoApprove determines if an action should be auto-approved
// Based on risk score and user mode settings
func (ap *ActionProposal) ShouldAutoApprove(userMode *UserMode) bool {
	// MANUAL mode never auto-approves
	if userMode != nil && userMode.Mode == ModeTypeManual {
		return false
	}

	// AUTO mode with low risk (< 30) auto-approves
	return ap.RiskScore < 30
}

// Goal represents a natural language goal injected by the user
// The Agentic Planner converts goals into atomic action proposals
type Goal struct {
	ID        string     `json:"id"`
	GoalText  string     `json:"goal"`
	Status    GoalStatus `json:"status"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
}

// GoalStatus represents the planning state of a goal
type GoalStatus string

const (
	GoalStatusActive    GoalStatus = "ACTIVE"    // Ready for planner to process
	GoalStatusPlanning  GoalStatus = "PLANNING"  // Planner is generating steps
	GoalStatusExecuting GoalStatus = "EXECUTING" // Actions are being executed
	GoalStatusCompleted GoalStatus = "COMPLETED" // Goal fully achieved
	GoalStatusFailed    GoalStatus = "FAILED"    // Goal could not be completed
)

// NewGoal creates a new goal with a generated UUID
func NewGoal(goalText string) *Goal {
	now := time.Now()
	return &Goal{
		ID:        uuid.New().String(),
		GoalText:  goalText,
		Status:    GoalStatusActive,
		CreatedAt: now,
		UpdatedAt: now,
	}
}

// AppState represents the global consciousness state of Engram
type AppState string

const (
	// AppStateActive - Full power: Perception (screen + files) + Agency (mouse/keyboard)
	AppStateActive AppState = "ACTIVE"

	// AppStateShadow - Learning mode: Perception only, no agency (safe default)
	AppStateShadow AppState = "SHADOW"

	// AppStatePaused - Privacy mode: Everything off
	AppStatePaused AppState = "PAUSED"
)

// IsValid checks if the app state is one of the valid states
func (s AppState) IsValid() bool {
	switch s {
	case AppStateActive, AppStateShadow, AppStatePaused:
		return true
	default:
		return false
	}
}
