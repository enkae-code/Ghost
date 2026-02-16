// Author: Enkae (enkae.dev@pm.me)
package service

import (
	"context"
	"encoding/json"
	"fmt"

	"ghost/kernel/internal/adapter"
	"ghost/kernel/internal/domain"
	"ghost/kernel/internal/protocol"
)

// LegacyBridge adapts the old TCP Gateway interface to the new GhostService/Kernel logic.
type LegacyBridge struct {
	ghostService *GhostService
	memoryRepo   *adapter.SQLiteRepository
}

// NewLegacyBridge creates a new bridge instance
func NewLegacyBridge(gs *GhostService, mr *adapter.SQLiteRepository) *LegacyBridge {
	return &LegacyBridge{
		ghostService: gs,
		memoryRepo:   mr,
	}
}

// RequestApproval adapts legacy ExecApprovalRequest to GhostService.RequestPermission
func (b *LegacyBridge) RequestApproval(ctx context.Context, req *protocol.ExecApprovalRequestParams) (*protocol.ExecApprovalResult, error) {
	// Unmarshal LegacyActions
	var legacyActions []protocol.LegacyAction
	if err := json.Unmarshal(req.Actions, &legacyActions); err != nil {
		return nil, fmt.Errorf("failed to unmarshal actions: %w", err)
	}

	// Convert to pb.Action
	var pbActions []*protocol.Action
	for _, la := range legacyActions {
		// Unmarshal Payload to map[string]interface{} then convert to map[string]string
		var payloadMap map[string]interface{}
		if len(la.Payload) > 0 {
			if err := json.Unmarshal(la.Payload, &payloadMap); err != nil {
				// If not a JSON object, might be raw string or empty
			}
		}

		strPayload := make(map[string]string)
		for k, v := range payloadMap {
			strPayload[k] = fmt.Sprintf("%v", v)
		}

		// Also add Target if present (legacy field)
		if la.Target != "" {
			strPayload["target"] = la.Target
		}

		// Map Legacy 'path' from payload directly if present and missing in map (sometimes inconsistent)
		if path, ok := payloadMap["path"]; ok {
			strPayload["path"] = fmt.Sprintf("%v", path)
		}

		pbActions = append(pbActions, &protocol.Action{
			Type:    la.Type,
			Payload: strPayload,
		})
	}

	pbReq := &protocol.PermissionRequest{
		Intent:  req.Intent,
		Actions: pbActions,
		TraceId: req.TraceID,
	}

	resp, err := b.ghostService.RequestPermission(ctx, pbReq)
	if err != nil {
		return nil, err
	}

	return &protocol.ExecApprovalResult{
		RequestID:  req.RequestID,
		Approved:   resp.Approved,
		Reason:     resp.Reason,
		TrustScore: int(resp.TrustScore),
	}, nil
}

// ResolveApproval adapts legacy ExecApprovalResolve to GhostService.ApproveAction
func (b *LegacyBridge) ResolveApproval(ctx context.Context, req *protocol.ExecApprovalResolveParams) error {
	pbReq := &protocol.ApprovalDecision{
		ActionId: req.RequestID, // Legacy RequestID maps to ActionID/ProposalID
		Approved: req.Approved,
	}
	_, err := b.ghostService.ApproveAction(ctx, pbReq)
	return err
}

// Store adapts legacy MemoryStore to SQLiteRepository.Save
func (b *LegacyBridge) Store(ctx context.Context, req *protocol.MemoryStoreParams) (*protocol.MemoryStoreResult, error) {
	// Map to Artifact using "memory" type
	// Content = Value
	// Classification = Key
	// Summary = Context

	// Dummy bounding box
	bbox := domain.BoundingBox{Left: 0, Top: 0, Right: 0, Bottom: 0}

	// Hack: Use ArtifactType("memory") - explicitly casting string
	artifact := domain.NewArtifact(domain.ArtifactType("memory"), req.Value, bbox)
	artifact.Classification = req.Key
	artifact.Summary = req.Context

	if err := b.memoryRepo.Save(ctx, artifact); err != nil {
		return nil, err
	}

	// Always persist Key and Context (classification, summary); embedding if provided (Save doesn't handle these)
	var embeddingJSON string
	if len(req.Vector) > 0 {
		b, _ := json.Marshal(req.Vector)
		embeddingJSON = string(b)
	}
	if err := b.memoryRepo.UpdateArtifact(ctx, artifact.ID, req.Key, req.Context, embeddingJSON); err != nil {
		fmt.Printf("[LEGACY] Failed to update metadata for memory %s: %v\n", artifact.ID, err)
	}

	return &protocol.MemoryStoreResult{Success: true, ArtifactID: artifact.ID}, nil
}

// Search adapts legacy MemorySearch to SQLiteRepository.SearchArtifacts
func (b *LegacyBridge) Search(ctx context.Context, req *protocol.MemorySearchParams) (*protocol.MemorySearchResult, error) {
	if len(req.Vector) == 0 {
		// Vector search requires vector. If only query string is provided,
		// we would need an embedding service here, which we don't have.
		// Return empty for now.
		return &protocol.MemorySearchResult{Artifacts: []protocol.MemoryArtifact{}}, nil
	}

	artifacts, err := b.memoryRepo.SearchArtifacts(ctx, req.Vector, req.Limit)
	if err != nil {
		return nil, err
	}

	var results []protocol.MemoryArtifact
	for _, a := range artifacts {
		results = append(results, protocol.MemoryArtifact{
			ID:        a.ID,
			Key:       a.Classification,
			Value:     a.Content,
			Context:   a.Summary,
			CreatedAt: a.Timestamp,
		})
	}
	return &protocol.MemorySearchResult{Artifacts: results}, nil
}
