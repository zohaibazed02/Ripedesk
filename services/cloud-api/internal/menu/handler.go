package menu

import (
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

func (h *Handler) GetBranchCatalog(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.ClaimsFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required", nil)
		return
	}
	branchID := r.PathValue("branchId")
	catalog, err := h.svc.GetBranchCatalog(r.Context(), claims.TenantID, branchID)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "CATALOG_FETCH_FAILED", "Failed to load branch catalog", nil)
		return
	}

	w.Header().Set("ETag", catalog.ETag)
	w.Header().Set("Cache-Control", "private, max-age=30, must-revalidate")
	if r.Header.Get("If-None-Match") == catalog.ETag {
		w.WriteHeader(http.StatusNotModified)
		return
	}

	httpx.WriteJSON(w, http.StatusOK, catalog, map[string]string{"etag": catalog.ETag})
}

func (h *Handler) UpsertMenuItem(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.ClaimsFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required", nil)
		return
	}
	branchID := r.PathValue("branchId")
	var in UpsertMenuItemInput
	if err := httpx.DecodeJSON(r, &in); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "INVALID_PAYLOAD", "Invalid menu item payload", err.Error())
		return
	}
	item, err := h.svc.UpsertMenuItem(r.Context(), claims.TenantID, branchID, in)
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "MENU_ITEM_UPSERT_FAILED", "Failed to save menu item", err.Error())
		return
	}
	httpx.WriteJSON(w, http.StatusOK, item, nil)
}
