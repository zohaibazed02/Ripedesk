package hr

import (
	"net/http"
	"time"

	"github.com/hospitality-os/cloud-api/internal/platform/auth"
	"github.com/hospitality-os/cloud-api/internal/platform/db"
	"github.com/hospitality-os/cloud-api/internal/platform/httpx"
	"github.com/jackc/pgx/v5"
)

type RosterShift struct {
	ID            string    `json:"id,omitempty"`
	UserID        string    `json:"user_id"`
	BranchID      string    `json:"branch_id"`
	ShiftStart    time.Time `json:"shift_start"`
	ShiftEnd      time.Time `json:"shift_end"`
	BreakMinutes  int       `json:"scheduled_break_minutes"`
	Station       string    `json:"station_assignment,omitempty"`
	Status        string    `json:"status,omitempty"`
}

// HasRosterOverlap returns true if candidate overlaps any existing shift for the same user across any branch.
func HasRosterOverlap(existing []RosterShift, candidate RosterShift) bool {
	for _, s := range existing {
		if s.UserID != candidate.UserID {
			continue
		}
		if candidate.ShiftStart.Before(s.ShiftEnd) && candidate.ShiftEnd.After(s.ShiftStart) {
			return true
		}
	}
	return false
}

type Handler struct {
	pool *db.Pool
}

func NewHandler(pool *db.Pool) *Handler {
	return &Handler{pool: pool}
}

func (h *Handler) ListRosters(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.ClaimsFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required", nil)
		return
	}
	items := []RosterShift{}
	err := h.pool.WithTenantTx(r.Context(), claims.TenantID, func(tx pgx.Tx) error {
		rows, err := tx.Query(r.Context(), `
			SELECT id, user_id, branch_id, shift_start, shift_end
			FROM staff_rosters
			ORDER BY shift_start ASC
			LIMIT 200
		`)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var s RosterShift
			if err := rows.Scan(&s.ID, &s.UserID, &s.BranchID, &s.ShiftStart, &s.ShiftEnd); err != nil {
				return err
			}
			s.Status = "published"
			items = append(items, s)
		}
		return rows.Err()
	})
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "ROSTERS_LIST_FAILED", err.Error(), nil)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, items, map[string]int{"count": len(items)})
}

func (h *Handler) CreateRoster(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.ClaimsFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required", nil)
		return
	}
	var in RosterShift
	if err := httpx.DecodeJSON(r, &in); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "INVALID_PAYLOAD", err.Error(), nil)
		return
	}
	if !in.ShiftEnd.After(in.ShiftStart) {
		httpx.WriteError(w, http.StatusUnprocessableEntity, "INVALID_SHIFT_RANGE", "shift_end must be after shift_start", nil)
		return
	}

	var id string
	err := h.pool.WithTenantTx(r.Context(), claims.TenantID, func(tx pgx.Tx) error {
		var overlap bool
		_ = tx.QueryRow(r.Context(), `
			SELECT EXISTS(
				SELECT 1 FROM staff_rosters
				WHERE user_id = $1 AND shift_start < $3 AND shift_end > $2
			)
		`, in.UserID, in.ShiftStart, in.ShiftEnd).Scan(&overlap)
		if overlap {
			return pgx.ErrTxCommitRollback
		}
		return tx.QueryRow(r.Context(), `
			INSERT INTO staff_rosters (tenant_id, branch_id, user_id, shift_start, shift_end)
			VALUES ($1, $2, $3, $4, $5)
			RETURNING id
		`, claims.TenantID, in.BranchID, in.UserID, in.ShiftStart, in.ShiftEnd).Scan(&id)
	})
	if err != nil {
		httpx.WriteError(w, http.StatusConflict, "ROSTER_OVERLAP_OR_ERROR", "Shift overlaps with an existing shift or failed to save", nil)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, map[string]string{"id": id, "status": "published"}, nil)
}
