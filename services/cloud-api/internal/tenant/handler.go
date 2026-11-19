package tenant

import (
	"errors"
	"net/http"

	"github.com/hospitality-os/cloud-api/internal/platform/auth"
	"github.com/hospitality-os/cloud-api/internal/platform/httpx"
)

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	var in LoginInput
	if err := httpx.DecodeJSON(r, &in); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "INVALID_PAYLOAD", "Invalid JSON request body", err.Error())
		return
	}
	res, err := h.svc.Login(r.Context(), in)
	if err != nil {
		if errors.Is(err, ErrInvalidCredentials) {
			httpx.WriteError(w, http.StatusUnauthorized, "INVALID_CREDENTIALS", "Invalid tenant, email, or password", nil)
			return
		}
		httpx.WriteError(w, http.StatusInternalServerError, "LOGIN_FAILED", "Failed to authenticate user", nil)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, res, nil)
}

func (h *Handler) VerifyPIN(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.ClaimsFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required", nil)
		return
	}
	var in PINVerifyInput
	if err := httpx.DecodeJSON(r, &in); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "INVALID_PAYLOAD", "Invalid JSON request body", err.Error())
		return
	}
	res, err := h.svc.VerifyPIN(r.Context(), claims.TenantID, in)
	if err != nil {
		switch {
		case errors.Is(err, ErrInvalidPIN):
			httpx.WriteError(w, http.StatusUnauthorized, "INVALID_PIN", "Staff PIN is invalid", nil)
		case errors.Is(err, ErrOverrideRequired):
			httpx.WriteError(w, http.StatusForbidden, "OVERRIDE_DENIED", "Staff member does not have manager override permission", nil)
		default:
			httpx.WriteError(w, http.StatusInternalServerError, "PIN_VERIFY_FAILED", "Failed to verify staff PIN", nil)
		}
		return
	}
	httpx.WriteJSON(w, http.StatusOK, res, nil)
}

func (h *Handler) EnrollDevice(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.ClaimsFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required", nil)
		return
	}
	var in EnrollDeviceInput
	if err := httpx.DecodeJSON(r, &in); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "INVALID_PAYLOAD", "Invalid JSON request body", err.Error())
		return
	}
	res, err := h.svc.EnrollDevice(r.Context(), claims.TenantID, in)
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "ENROLL_FAILED", "Failed to enroll device", err.Error())
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, res, nil)
}

func (h *Handler) ListMyBranches(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.ClaimsFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required", nil)
		return
	}
	branches, err := h.svc.ListBranches(r.Context(), claims.TenantID)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "BRANCHES_LIST_FAILED", "Failed to list tenant branches", nil)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, branches, map[string]int{"count": len(branches)})
}

func (h *Handler) Heartbeat(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.ClaimsFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required", nil)
		return
	}
	branchID := r.PathValue("branchId")
	var in HeartbeatInput
	if err := httpx.DecodeJSON(r, &in); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "INVALID_PAYLOAD", "Invalid JSON request body", err.Error())
		return
	}
	branch, err := h.svc.RecordHeartbeat(r.Context(), claims.TenantID, branchID, in)
	if err != nil {
		httpx.WriteError(w, http.StatusNotFound, "HEARTBEAT_FAILED", "Branch not found or heartbeat update failed", nil)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, branch, nil)
}
