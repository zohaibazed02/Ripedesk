package sync

import (
	"errors"
	"net/http"

	"github.com/hospitality-os/cloud-api/internal/platform/auth"
	"github.com/hospitality-os/cloud-api/internal/platform/httpx"
	"github.com/jackc/pgx/v5"
)

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) Push(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.ClaimsFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required", nil)
		return
	}
	branchID := r.PathValue("branchId")

	var req PushBatchRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "INVALID_PAYLOAD", "Invalid sync push batch payload", err.Error())
		return
	}

	res, err := h.svc.PushBatch(r.Context(), claims.TenantID, branchID, req)
	if err != nil {
		if errors.Is(err, ErrBatchTooLarge) {
			httpx.WriteError(w, http.StatusRequestEntityTooLarge, "BATCH_TOO_LARGE", err.Error(), nil)
			return
		}
		httpx.WriteError(w, http.StatusInternalServerError, "SYNC_PUSH_FAILED", "Failed to ingest sync batch", err.Error())
		return
	}

	httpx.WriteJSON(w, http.StatusOK, res, map[string]int{"ack_count": len(res.Acks)})
}

func (h *Handler) Pull(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.ClaimsFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required", nil)
		return
	}
	branchID := r.PathValue("branchId")
	sinceCursor := r.URL.Query().Get("since_cursor")

	res, err := h.svc.PullEvents(r.Context(), claims.TenantID, branchID, sinceCursor)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "SYNC_PULL_FAILED", "Failed to pull cloud events", err.Error())
		return
	}
	httpx.WriteJSON(w, http.StatusOK, res, map[string]int{"count": len(res.Events)})
}

func (h *Handler) ListReviewQueue(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.ClaimsFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required", nil)
		return
	}
	branchID := r.PathValue("branchId")

	items, err := h.svc.ListReviewQueue(r.Context(), claims.TenantID, branchID)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "REVIEW_QUEUE_FAILED", "Failed to list sync review queue", err.Error())
		return
	}
	httpx.WriteJSON(w, http.StatusOK, items, map[string]int{"count": len(items)})
}

func (h *Handler) ResolveReviewItem(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.ClaimsFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required", nil)
		return
	}
	branchID := r.PathValue("branchId")
	itemID := r.PathValue("id")

	var in ResolveReviewInput
	if err := httpx.DecodeJSON(r, &in); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "INVALID_PAYLOAD", "Invalid resolve payload", err.Error())
		return
	}

	if err := h.svc.ResolveReviewItem(r.Context(), claims.TenantID, branchID, itemID, claims.UserID, in.Status); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			httpx.WriteError(w, http.StatusNotFound, "NOT_FOUND", "Sync review queue item not found", nil)
			return
		}
		httpx.WriteError(w, http.StatusBadRequest, "RESOLVE_FAILED", err.Error(), nil)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]string{"id": itemID, "status": in.Status}, nil)
}
