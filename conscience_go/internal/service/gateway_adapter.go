// Author: Enkae (enkae.dev@pm.me)
package service

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"ghost/kernel/internal/domain"
	"ghost/kernel/internal/protocol"
)

// GatewayAdapter bridges the JSON-RPC Gateway with the GhostService
type GatewayAdapter struct {
	service *GhostService
}

// NewGatewayAdapter creates a new adapter
func NewGatewayAdapter(service *GhostService) *GatewayAdapter {
	return &GatewayAdapter{service: service}
}

// RequestApproval implements gateway.ApprovalHandler
func (g *GatewayAdapter) RequestApproval(ctx context.Context, req *protocol.ExecApprovalRequestParams) (*protocol.ExecApprovalResult, error) {
	// 1. Parse Actions from JSON
	// brain_python sends flat actions: [{"type": "WRITE", "path": "foo", ...}]
	var rawActions []map[string]interface{}
	if err := json.Unmarshal(req.Actions, &rawActions); err != nil {
		return nil, fmt.Errorf("failed to parse actions: %w", err)
	}

	// 2. Convert to Protobuf Actions
	var pbActions []*protocol.Action
	for _, raw := range rawActions {
		actionType, ok := raw["type"].(string)
		if !ok {
			continue // Skip invalid actions
		}

		payload := make(map[string]string)
		for k, v := range raw {
			if k == "type" {
				continue
			}
			// Convert value to string
			payload[k] = fmt.Sprintf("%v", v)
		}

		pbActions = append(pbActions, &protocol.Action{
			Type:    actionType,
			Payload: payload,
		})
	}

	// 3. Call Service
	permReq := &protocol.PermissionRequest{
		Intent:  req.Intent,
		Actions: pbActions,
		TraceId: req.TraceID,
	}

	resp, err := g.service.RequestPermission(ctx, permReq)
	if err != nil {
		return nil, err
	}

	// 4. Map Response
	return &protocol.ExecApprovalResult{
		RequestID:  req.RequestID,
		Approved:   resp.Approved,
		Reason:     resp.Reason,
		TrustScore: int(resp.TrustScore),
	}, nil
}

// ResolveApproval implements gateway.ApprovalHandler
func (g *GatewayAdapter) ResolveApproval(ctx context.Context, req *protocol.ExecApprovalResolveParams) error {
	decision := &protocol.ApprovalDecision{
		ActionId: req.RequestID, // Assuming RequestID maps to ActionID in this context
		Approved: req.Approved,
	}

	_, err := g.service.ApproveAction(ctx, decision)
	return err
}

// Store implements gateway.MemoryHandler
func (g *GatewayAdapter) Store(ctx context.Context, req *protocol.MemoryStoreParams) (*protocol.MemoryStoreResult, error) {
	// Convert params to Artifact
	artifact := domain.Artifact{
		ID:             fmt.Sprintf("mem_%d", time.Now().UnixNano()),
		Type:           domain.ArtifactTypeText,
		Content:        req.Value,
		Timestamp:      time.Now(),
		Classification: req.Key,     // Storing Key in Classification for retrieval
		Summary:        req.Context, // Storing Context in Summary
	}

	// Save artifact
	if err := g.service.MemoryRepo.Save(ctx, artifact); err != nil {
		return &protocol.MemoryStoreResult{Success: false}, err
	}

	// Update with embedding if provided
	if len(req.Vector) > 0 {
		embeddingJSON, _ := json.Marshal(req.Vector)
		// UpdateArtifact enriches the artifact with embedding
		if err := g.service.MemoryRepo.UpdateArtifact(ctx, artifact.ID, req.Key, req.Context, string(embeddingJSON)); err != nil {
			return &protocol.MemoryStoreResult{Success: false}, err
		}
	}

	return &protocol.MemoryStoreResult{Success: true, ArtifactID: artifact.ID}, nil
}

// Search implements gateway.MemoryHandler
func (g *GatewayAdapter) Search(ctx context.Context, req *protocol.MemorySearchParams) (*protocol.MemorySearchResult, error) {
	if len(req.Vector) == 0 {
		return &protocol.MemorySearchResult{Artifacts: []protocol.MemoryArtifact{}}, nil
	}

	artifacts, err := g.service.MemoryRepo.SearchArtifacts(ctx, req.Vector, req.Limit)
	if err != nil {
		return nil, err
	}

	var results []protocol.MemoryArtifact
	for _, a := range artifacts {
		results = append(results, protocol.MemoryArtifact{
			ID:        a.ID,
			Key:       a.Classification, // We stored Key in Classification
			Value:     a.Content,
			Context:   a.Summary, // We stored Context in Summary
			CreatedAt: a.Timestamp,
		})
	}

	return &protocol.MemorySearchResult{Artifacts: results}, nil
}
