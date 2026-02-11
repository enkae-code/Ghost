// Author: Enkae (enkae.dev@pm.me)
package service

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	"encoding/json"

	"ghost/kernel/internal/adapter"
	"ghost/kernel/internal/conscience"
	"ghost/kernel/internal/domain"
	pb "ghost/kernel/internal/protocol"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
)

// GhostService implements the NervousSystemServer interface.
type GhostService struct {
	pb.UnimplementedNervousSystemServer

	// ActionRepo is the repository for action-related data.
	ActionRepo *adapter.ActionRepository
	// IntentRepo is the repository for intent history.
	IntentRepo *adapter.IntentHistoryRepository
	// MemoryRepo is the repository for system memory (SQLite).
	MemoryRepo *adapter.SQLiteRepository
	// StateRepo is the repository for system state.
	StateRepo *adapter.StateRepository
	// Safety is the safety checker for intents and actions.
	Safety *SafetyChecker
	// Validator is the core conscience kernel validator.
	Validator *conscience.Validator

	// focusMu protects focusState.
	focusMu sync.RWMutex
	// focusState stores the current focus information from the Sentinel.
	focusState *pb.FocusState

	// actionChan is a buffered channel for sending action commands to the Body.
	actionChan chan *pb.ActionCommand
}

// NewGhostService creates the service with dependencies.
func NewGhostService(
	actionRepo *adapter.ActionRepository,
	intentRepo *adapter.IntentHistoryRepository,
	memoryRepo *adapter.SQLiteRepository,
	stateRepo *adapter.StateRepository,
) *GhostService {
	return &GhostService{
		ActionRepo: actionRepo,
		IntentRepo: intentRepo,
		MemoryRepo: memoryRepo,
		StateRepo:  stateRepo,
		Safety:     NewSafetyChecker(DefaultSafetyConfig()), // Use strict defaults by default
		Validator:  conscience.NewValidator(),
		focusState: &pb.FocusState{WindowTitle: "Unknown"},
		actionChan: make(chan *pb.ActionCommand, 100), // Buffer for safety
	}
}

// --- SENSORY INPUT ---

func (s *GhostService) ReportFocus(stream pb.NervousSystem_ReportFocusServer) error {
	for {
		focus, err := stream.Recv()
		if err != nil {
			slog.Warn("Focus stream ended", "error", err)
			return err
		}

		s.focusMu.Lock()
		s.focusState = focus
		s.focusMu.Unlock()

		slog.Debug("Focus updated", "window", focus.WindowTitle, "process", focus.ProcessName)
	}
}

// --- COGNITION ---

// RequestPermission evaluates a request from the Brain to perform actions.
func (s *GhostService) RequestPermission(ctx context.Context, req *pb.PermissionRequest) (*pb.PermissionResponse, error) {
	slog.Info("Permission Request", "intent", req.Intent, "trace_id", req.TraceId)

	// 1. Check Current Focus (Context Awareness)
	s.focusMu.RLock()
	currentWindow := s.focusState.WindowTitle
	s.focusMu.RUnlock()

	// 2. Safety Check (Policy Engine)
	isDangerous, kw := s.Safety.IsDangerous(req.Intent)
	if isDangerous {
		slog.Warn("Safety Violation", "intent", req.Intent, "keyword", kw)
		return &pb.PermissionResponse{
			Approved: false,
			Reason:   "Violates Safety Policy: Blocked Keyword '" + kw + "'",
		}, nil
	}

	// 3. Action Validation (Via Conscience Kernel)
	legacyActions := make([]pb.LegacyAction, len(req.Actions))
	for i, action := range req.Actions {
		// Marshal payload map to JSON
		payloadBytes, _ := json.Marshal(action.Payload)
		legacyActions[i] = pb.LegacyAction{
			Type:      action.Type,
			Payload:   payloadBytes,
			RiskLevel: pb.RiskLevelNone, // Validator will calculate
		}
	}

	validationReq := &pb.ActionValidationRequest{
		RequestID:      req.TraceId,
		Intent:         req.Intent,
		Actions:        legacyActions,
		ExpectedWindow: "",    // TODO: Extract from intent if possible
		Override:       false, // Brain cannot override on its own
		TraceID:        req.TraceId,
	}

	// Update validator focus context
	s.Validator.SetFocusedWindow(currentWindow)

	validationResult := s.Validator.ValidateAction(ctx, validationReq)
	if !validationResult.Valid || validationResult.Blocked {
		slog.Warn("Conscience Blocked Action",
			"reason", validationResult.Reason,
			"risk_level", validationResult.RiskLevel,
			"trace_id", req.TraceId,
		)
		return &pb.PermissionResponse{
			Approved:   false,
			Reason:     "Conscience Violation: " + validationResult.Reason,
			TrustScore: int32(validationResult.TrustScore),
		}, nil
	}

	// 4. Log Intent
	// Note: We perform this async or ignore error to not block latency
	go func() {
		_ = s.IntentRepo.RecordSuccess(context.Background(), req.Intent, currentWindow, "")
	}()

	// 5. Enqueue approved actions to Body stream
	for i, action := range req.Actions {
		cmd := &pb.ActionCommand{
			CommandId: fmt.Sprintf("%s-%d", req.TraceId, i),
			Action:    action,
		}
		select {
		case s.actionChan <- cmd:
			slog.Info("Action enqueued for Body", "id", cmd.CommandId, "type", action.Type)
		default:
			slog.Warn("Action channel full, dropping", "id", cmd.CommandId)
		}
	}

	return &pb.PermissionResponse{
		Approved:   true,
		TrustScore: 85,
	}, nil
}

// --- MOTOR CONTROL ---

func (s *GhostService) StreamActions(_ *emptypb.Empty, stream pb.NervousSystem_StreamActionsServer) error {
	slog.Info("Sentinel connected to Action Stream")
	for cmd := range s.actionChan {
		if err := stream.Send(cmd); err != nil {
			slog.Error("Failed to send action", "error", err)
			return err
		}
	}
	return nil
}

// --- HUMAN CONTROL PLANE (Gateway) ---

func (s *GhostService) GetSystemState(ctx context.Context, _ *emptypb.Empty) (*pb.SystemState, error) {
	// Fetch state from repo
	stateStr, err := s.StateRepo.GetState(ctx)
	if err != nil {
		return nil, status.Error(codes.Internal, "Failed to fetch state")
	}

	s.focusMu.RLock()
	activeFocus := s.focusState.WindowTitle
	s.focusMu.RUnlock()

	return &pb.SystemState{
		State:       string(stateStr),
		ActiveFocus: activeFocus,
	}, nil
}

func (s *GhostService) GetPendingApprovals(ctx context.Context, _ *emptypb.Empty) (*pb.PendingList, error) {
	actions, err := s.ActionRepo.GetPendingApprovals(ctx)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	// Convert domain model to proto model
	var protoItems []*pb.PendingItem
	for _, a := range actions {
		protoItems = append(protoItems, &pb.PendingItem{
			ActionId:  a.ID,
			Intent:    a.Intent,
			RiskScore: int32(a.RiskScore),
		})
	}

	return &pb.PendingList{Items: protoItems}, nil
}

func (s *GhostService) ApproveAction(ctx context.Context, req *pb.ApprovalDecision) (*pb.Ack, error) {
	actionStatus := domain.ActionProposalStatusRejected
	if req.Approved {
		actionStatus = domain.ActionProposalStatusApproved
	}

	if err := s.ActionRepo.UpdateActionStatus(ctx, req.ActionId, actionStatus); err != nil {
		return &pb.Ack{Success: false}, status.Error(codes.Internal, err.Error())
	}

	// If approved, we might want to enqueue it to s.actionChan here immediately
	// For now, we assume the Brain polls or streams "Approved" actions separately.

	return &pb.Ack{Success: true}, nil
}

func (s *GhostService) SetSystemMode(ctx context.Context, req *pb.ModeRequest) (*pb.Ack, error) {
	mode := domain.ModeTypeManual
	if req.Mode == "AUTO" {
		mode = domain.ModeTypeAuto
	}

	if err := s.ActionRepo.SetUserMode(ctx, req.Domain, mode); err != nil {
		return &pb.Ack{Success: false}, err
	}
	return &pb.Ack{Success: true}, nil
}
