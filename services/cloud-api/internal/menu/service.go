package menu

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
)

type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) Name() string {
	return "menu"
}

func (s *Service) GetBranchCatalog(ctx context.Context, tenantID, branchID string) (*BranchCatalog, error) {
	cat, err := s.repo.GetBranchCatalog(ctx, tenantID, branchID)
	if err != nil {
		return nil, err
	}
	raw, err := json.Marshal(cat)
	if err != nil {
		return nil, err
	}
	sum := sha256.Sum256(raw)
	cat.ETag = `"` + hex.EncodeToString(sum[:16]) + `"`
	return cat, nil
}

func (s *Service) UpsertMenuItem(ctx context.Context, tenantID, branchID string, in UpsertMenuItemInput) (*MenuItem, error) {
	return s.repo.UpsertMenuItem(ctx, tenantID, branchID, in)
}
