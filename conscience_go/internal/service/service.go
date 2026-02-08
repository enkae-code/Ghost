// Author: Enkae (enkae.dev@pm.me)
package service

import (
	"context"
	"log/slog"
	"sync"

	"ghost/kernel/internal/adapter"
	"ghost/kernel/internal/domain"
	pb "ghost/kernel/internal/protocol"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
)

// GhostService implements the NervousSystemServer interface.
type GhostService struct {
	pb.UnimplementedNervousSystemServer

	// Dependencies
	ActionRepo *adapter.ActionRepository
	IntentRepo *adapter.IntentHistoryRepository
	MemoryRepo *adapter.SQLiteRepository
	StateRepo  *adapter.StateRepository
	Safety     *SafetyChecker

	// Live State (Thread-Safe)
	focusMu    sync.RWMutex
	focusState *pb.FocusState

	// Action Stream (for sending commands to Body)
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

	// 3. Log Intent
	// Note: We perform this async or ignore error to not block latency
	go func() {
		// Adapt this call to your specific IntentRepo method signature
		_ = s.IntentRepo.RecordSuccess(context.Background(), req.Intent, currentWindow, "")
	}()

	return &pb.PermissionResponse{
		Approved:   true,
		TrustScore: 85, // Mock score for now
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
