package tenant

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"time"

	"github.com/hospitality-os/cloud-api/internal/platform/auth"
)

var (
	ErrInvalidCredentials = errors.New("invalid email, password, or tenant")
	ErrInvalidPIN         = errors.New("invalid staff PIN")
	ErrOverrideRequired   = errors.New("manager override permission required")
)

type Service struct {
	repo        *Repository
	tokens      *auth.TokenManager
	heartbeatTT int
}

func NewService(repo *Repository, tokens *auth.TokenManager, heartbeatTTLSeconds int) *Service {
	if heartbeatTTLSeconds <= 0 {
		heartbeatTTLSeconds = 45
	}
	return &Service{
		repo:        repo,
		tokens:      tokens,
		heartbeatTT: heartbeatTTLSeconds,
	}
}

func (s *Service) Name() string {
	return "tenant"
}

type LoginInput struct {
	TenantSlug string `json:"tenant_slug"`
	Email      string `json:"email"`
	Password   string `json:"password"`
}

type LoginResult struct {
	AccessToken  string          `json:"access_token"`
	User         UserProfile     `json:"user"`
	Entitlements map[string]bool `json:"entitlements"`
	Branches     []BranchSummary `json:"branches"`
}

type UserProfile struct {
	ID          string   `json:"id"`
	TenantID    string   `json:"tenant_id"`
	FullName    string   `json:"full_name"`
	Email       string   `json:"email"`
	Role        string   `json:"role"`
	CanOverride bool     `json:"can_override"`
	BranchIDs   []string `json:"branch_ids"`
}

func (s *Service) Login(ctx context.Context, in LoginInput) (*LoginResult, error) {
	t, err := s.repo.FindTenantBySlug(ctx, in.TenantSlug)
	if err != nil {
		return nil, ErrInvalidCredentials
	}
	u, err := s.repo.FindUserByEmail(ctx, t.ID, in.Email)
	if err != nil || !u.IsActive {
		return nil, ErrInvalidCredentials
	}
	if auth.HashSecret(t.ID, in.Password) != u.PasswordHash {
		return nil, ErrInvalidCredentials
	}

	token, err := s.tokens.Sign(auth.Claims{
		UserID:       u.ID,
		TenantID:     t.ID,
		Role:         u.RoleCode,
		BranchIDs:    u.BranchIDs,
		CanOverride:  u.CanOverride,
		Entitlements: t.Entitlements,
	})
	if err != nil {
		return nil, err
	}

	branches, err := s.repo.ListBranches(ctx, t.ID)
	if err != nil {
		return nil, err
	}

	return &LoginResult{
		AccessToken: token,
		User: UserProfile{
			ID:          u.ID,
			TenantID:    t.ID,
			FullName:    u.FullName,
			Email:       u.Email,
			Role:        u.RoleCode,
			CanOverride: u.CanOverride,
			BranchIDs:   u.BranchIDs,
		},
		Entitlements: t.Entitlements,
		Branches:     branches,
	}, nil
}

type PINVerifyInput struct {
	PIN             string `json:"pin"`
	BranchID        string `json:"branch_id"`
	RequireOverride bool   `json:"require_override"`
	Reason          string `json:"reason"`
}

type PINVerifyResult struct {
	Verified    bool   `json:"verified"`
	UserID      string `json:"user_id"`
	FullName    string `json:"full_name"`
	Role        string `json:"role"`
	CanOverride bool   `json:"can_override"`
}

func (s *Service) VerifyPIN(ctx context.Context, tenantID string, in PINVerifyInput) (*PINVerifyResult, error) {
	pinHash := auth.HashSecret(tenantID, in.PIN)
	u, err := s.repo.FindUserByPINHash(ctx, tenantID, pinHash)
	if err != nil {
		return nil, ErrInvalidPIN
	}
	if in.RequireOverride && !u.CanOverride {
		return nil, ErrOverrideRequired
	}

	_ = s.repo.RecordAuditEvent(ctx, tenantID, in.BranchID, u.ID, "staff.pin_verified", map[string]any{
		"user_id":          u.ID,
		"role":             u.RoleCode,
		"require_override": in.RequireOverride,
		"reason":           in.Reason,
		"verified_at":      time.Now().UTC().Format(time.RFC3339),
	})

	return &PINVerifyResult{
		Verified:    true,
		UserID:      u.ID,
		FullName:    u.FullName,
		Role:        u.RoleCode,
		CanOverride: u.CanOverride,
	}, nil
}

type EnrollDeviceInput struct {
	BranchID            string `json:"branch_id"`
	DeviceName          string `json:"device_name"`
	DeviceType          string `json:"device_type"`
	HardwareFingerprint string `json:"hardware_fingerprint"`
}

type EnrollDeviceResult struct {
	Device           DeviceRecord    `json:"device"`
	DeviceCredential string          `json:"device_credential"`
	BranchConfig     json.RawMessage `json:"branch_config"`
}

func (s *Service) EnrollDevice(ctx context.Context, tenantID string, in EnrollDeviceInput) (*EnrollDeviceResult, error) {
	rawBytes := make([]byte, 32)
	if _, err := rand.Read(rawBytes); err != nil {
		return nil, err
	}
	credential := hex.EncodeToString(rawBytes)
	keyHash := auth.HashSecret(tenantID, credential)

	dev, branchCfg, err := s.repo.EnrollDevice(
		ctx,
		tenantID,
		in.BranchID,
		in.DeviceName,
		in.DeviceType,
		in.HardwareFingerprint,
		keyHash,
	)
	if err != nil {
		return nil, err
	}

	return &EnrollDeviceResult{
		Device:           *dev,
		DeviceCredential: credential,
		BranchConfig:     branchCfg,
	}, nil
}

func (s *Service) ListBranches(ctx context.Context, tenantID string) ([]BranchSummary, error) {
	return s.repo.ListBranches(ctx, tenantID)
}

type HeartbeatInput struct {
	PendingSyncCount  int        `json:"pending_sync_count"`
	OldestPendingTxAt *time.Time `json:"oldest_pending_tx_at"`
}

func (s *Service) RecordHeartbeat(ctx context.Context, tenantID, branchID string, in HeartbeatInput) (*BranchSummary, error) {
	return s.repo.UpdateBranchHeartbeat(ctx, tenantID, branchID, s.heartbeatTT, in.PendingSyncCount, in.OldestPendingTxAt)
}
