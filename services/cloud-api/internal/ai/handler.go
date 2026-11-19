package ai

import (
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"time"

	"github.com/hospitality-os/cloud-api/internal/platform/auth"
	"github.com/hospitality-os/cloud-api/internal/platform/db"
	"github.com/hospitality-os/cloud-api/internal/platform/httpx"
)

// Handler serves the read-only AI Reporting Agent HTTP endpoints.
type Handler struct {
	agent *Agent
}

// NewHandler creates a new AI Reporting HTTP handler backed by PostgresReadOnlyDataSource.
func NewHandler(pool *db.Pool) *Handler {
	ds := NewPostgresReadOnlyDataSource(pool)
	registry := NewToolRegistry(ds)
	explainer := &ManagedLLMHTTPAdapter{
		APIURL:       os.Getenv("AI_LLM_API_URL"),
		APIKey:       os.Getenv("AI_LLM_API_KEY"),
		ModelVersion: os.Getenv("AI_MODEL_VERSION"),
		Fallback:     &GroundedAnalyticalExplainer{},
	}
	return &Handler{agent: NewAgent(registry, explainer)}
}

type askRequest struct {
	Question  string   `json:"question"`
	From      string   `json:"from"`
	To        string   `json:"to"`
	BranchIDs []string `json:"branch_ids"`
}

// Ask handles POST /api/v1/ai/ask
func (h *Handler) Ask(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.ClaimsFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required", nil)
		return
	}

	var req askRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Question == "" {
		httpx.WriteError(w, http.StatusBadRequest, "INVALID_REQUEST", "question is required", nil)
		return
	}

	today := time.Now().UTC().Format("2006-01-02")
	if req.From == "" {
		req.From = today
	}
	if req.To == "" {
		req.To = today
	}

	execCtx := ToolExecutionContext{
		TenantID:           claims.TenantID,
		ActorUserID:        claims.UserID,
		ActorRole:          claims.Role,
		AllowedBranchIDs:   claims.BranchIDs,
		RequestedBranchIDs: req.BranchIDs,
		FromDate:           req.From,
		ToDate:             req.To,
	}

	resp, err := h.agent.Ask(r.Context(), execCtx, req.Question)
	if err != nil {
		if errors.Is(err, ErrForbiddenBranchScope) {
			httpx.WriteError(w, http.StatusForbidden, "FORBIDDEN_BRANCH_SCOPE", err.Error(), nil)
			return
		}
		if errors.Is(err, ErrUnapprovedTool) {
			httpx.WriteError(w, http.StatusBadRequest, "UNAPPROVED_TOOL", err.Error(), nil)
			return
		}
		httpx.WriteError(w, http.StatusInternalServerError, "AI_AGENT_ERROR", err.Error(), nil)
		return
	}

	httpx.WriteJSON(w, http.StatusOK, resp, nil)
}

// DailyBriefing handles GET /api/v1/ai/daily-briefing?branch_id=
func (h *Handler) DailyBriefing(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.ClaimsFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required", nil)
		return
	}

	today := time.Now().UTC().Format("2006-01-02")
	var reqBranches []string
	if bID := r.URL.Query().Get("branch_id"); bID != "" {
		reqBranches = []string{bID}
	}

	execCtx := ToolExecutionContext{
		TenantID:           claims.TenantID,
		ActorUserID:        claims.UserID,
		ActorRole:          claims.Role,
		AllowedBranchIDs:   claims.BranchIDs,
		RequestedBranchIDs: reqBranches,
		FromDate:           today,
		ToDate:             today,
	}

	resp, err := h.agent.DailyBriefing(r.Context(), execCtx)
	if err != nil {
		if errors.Is(err, ErrForbiddenBranchScope) {
			httpx.WriteError(w, http.StatusForbidden, "FORBIDDEN_BRANCH_SCOPE", err.Error(), nil)
			return
		}
		httpx.WriteError(w, http.StatusInternalServerError, "AI_BRIEFING_ERROR", err.Error(), nil)
		return
	}

	httpx.WriteJSON(w, http.StatusOK, resp, nil)
}
