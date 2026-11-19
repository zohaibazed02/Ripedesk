package delivery

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/hospitality-os/cloud-api/internal/platform/auth"
	"github.com/hospitality-os/cloud-api/internal/platform/db"
	"github.com/hospitality-os/cloud-api/internal/platform/httpx"
	"github.com/jackc/pgx/v5"
	"github.com/shopspring/decimal"
)

var (
	ErrInvalidTripTransition = errors.New("invalid delivery trip state transition")
	ErrPODRequired           = errors.New("proof of delivery (pod_photo_url or recipient_name) is required to complete delivery")
)

const (
	TripUnassigned     = "unassigned"
	TripAssigned       = "assigned"
	TripPickedUp       = "picked_up"
	TripOutForDelivery = "out_for_delivery"
	TripDelivered      = "delivered"
	TripFailed         = "failed_delivery"
)

var allowedTripTransitions = map[string]map[string]bool{
	TripUnassigned:     {TripAssigned: true},
	TripAssigned:       {TripPickedUp: true, TripFailed: true},
	TripPickedUp:       {TripOutForDelivery: true, TripFailed: true},
	TripOutForDelivery: {TripDelivered: true, TripFailed: true},
}

func ValidateTripTransition(from, to string) error {
	if nexts, ok := allowedTripTransitions[from]; ok && nexts[to] {
		return nil
	}
	return fmt.Errorf("%w: %s -> %s", ErrInvalidTripTransition, from, to)
}

func ValidatePOD(photoURL, recipientName string) error {
	if strings.TrimSpace(photoURL) == "" && strings.TrimSpace(recipientName) == "" {
		return ErrPODRequired
	}
	return nil
}

type CODReconciliationResult struct {
	TripCount      int             `json:"trip_count"`
	TotalExpected  decimal.Decimal `json:"total_expected_cod"`
	TotalCollected decimal.Decimal `json:"total_collected_cod"`
	HandedOverCash decimal.Decimal `json:"handed_over_cash"`
	Variance       decimal.Decimal `json:"variance"` // handed_over - total_expected (negative = short)
}

func ComputeCODReconciliation(expected, collected []decimal.Decimal, handedOver decimal.Decimal) CODReconciliationResult {
	totExp := decimal.Zero
	for _, e := range expected {
		totExp = totExp.Add(e)
	}
	totCol := decimal.Zero
	for _, c := range collected {
		totCol = totCol.Add(c)
	}
	variance := handedOver.Sub(totExp).Round(4)
	return CODReconciliationResult{
		TripCount:      len(expected),
		TotalExpected:  totExp.Round(4),
		TotalCollected: totCol.Round(4),
		HandedOverCash: handedOver.Round(4),
		Variance:       variance,
	}
}

type Handler struct {
	pool *db.Pool
}

func NewHandler(pool *db.Pool) *Handler {
	return &Handler{pool: pool}
}

func (h *Handler) ListTrips(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.ClaimsFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required", nil)
		return
	}
	type tripRow struct {
		ID           string `json:"id"`
		OrderID      string `json:"order_id"`
		RiderUserID  string `json:"rider_user_id,omitempty"`
		Status       string `json:"status"`
		CODExpected  string `json:"cod_expected_amount"`
		CODCollected string `json:"cod_collected_amount"`
		PODPhotoURL  string `json:"pod_photo_url,omitempty"`
	}
	items := []tripRow{}
	err := h.pool.WithTenantTx(r.Context(), claims.TenantID, func(tx pgx.Tx) error {
		query := `
			SELECT id, order_id, COALESCE(rider_user_id::text, ''), status,
			       cod_expected_amount::text, cod_collected_amount::text, COALESCE(pod_photo_url, '')
			FROM delivery_trips
		`
		var args []any
		// Narrow rider scope: riders can ONLY view their own assigned trips
		if claims.Role == "rider" {
			query += ` WHERE rider_user_id = $1`
			args = append(args, claims.UserID)
		}
		query += ` ORDER BY created_at DESC LIMIT 100`

		rows, err := tx.Query(r.Context(), query, args...)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var t tripRow
			if err := rows.Scan(&t.ID, &t.OrderID, &t.RiderUserID, &t.Status, &t.CODExpected, &t.CODCollected, &t.PODPhotoURL); err != nil {
				return err
			}
			items = append(items, t)
		}
		return rows.Err()
	})
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "TRIPS_LIST_FAILED", err.Error(), nil)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, items, map[string]int{"count": len(items)})
}

func (h *Handler) AssignTrip(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.ClaimsFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required", nil)
		return
	}
	var in struct {
		BranchID          string          `json:"branch_id"`
		OrderID           string          `json:"order_id"`
		RiderUserID       string          `json:"rider_user_id"`
		CODExpectedAmount decimal.Decimal `json:"cod_expected_amount"`
	}
	if err := httpx.DecodeJSON(r, &in); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "INVALID_PAYLOAD", err.Error(), nil)
		return
	}
	var tripID string
	token := fmt.Sprintf("trk_%d_%s", time.Now().Unix(), in.OrderID)
	err := h.pool.WithTenantTx(r.Context(), claims.TenantID, func(tx pgx.Tx) error {
		return tx.QueryRow(r.Context(), `
			INSERT INTO delivery_trips (
				tenant_id, branch_id, order_id, rider_user_id, status,
				cod_expected_amount, tracking_token, tracking_expires_at
			)
			VALUES ($1, $2, $3, $4, 'assigned', $5::numeric, $6, NOW() + interval '6 hours')
			RETURNING id
		`, claims.TenantID, in.BranchID, in.OrderID, in.RiderUserID, in.CODExpectedAmount.StringFixed(4), token).Scan(&tripID)
	})
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "ASSIGN_TRIP_FAILED", err.Error(), nil)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, map[string]string{
		"trip_id":        tripID,
		"status":         TripAssigned,
		"tracking_token": token,
	}, nil)
}

func (h *Handler) UpdateTripStatus(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.ClaimsFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required", nil)
		return
	}
	tripID := r.PathValue("tripId")
	var in struct {
		ClientEventID      string          `json:"client_event_id"`
		BranchID           string          `json:"branch_id"`
		NextStatus         string          `json:"next_status"`
		PODPhotoURL        string          `json:"pod_photo_url"`
		RecipientName      string          `json:"recipient_name"`
		CODCollectedAmount decimal.Decimal `json:"cod_collected_amount"`
	}
	if err := httpx.DecodeJSON(r, &in); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "INVALID_PAYLOAD", err.Error(), nil)
		return
	}
	if in.NextStatus == TripDelivered {
		if err := ValidatePOD(in.PODPhotoURL, in.RecipientName); err != nil {
			httpx.WriteError(w, http.StatusUnprocessableEntity, "POD_REQUIRED", err.Error(), nil)
			return
		}
	}

	var duplicate bool
	err := h.pool.WithTenantTx(r.Context(), claims.TenantID, func(tx pgx.Tx) error {
		if in.ClientEventID != "" {
			var exists bool
			_ = tx.QueryRow(r.Context(), `SELECT EXISTS(SELECT 1 FROM processed_commands WHERE command_id = $1)`, in.ClientEventID).Scan(&exists)
			if exists {
				duplicate = true
				return nil
			}
		}

		var curStatus, assignedRider string
		if err := tx.QueryRow(r.Context(), `
			SELECT status, COALESCE(rider_user_id::text, '') FROM delivery_trips WHERE id = $1 FOR UPDATE
		`, tripID).Scan(&curStatus, &assignedRider); err != nil {
			return err
		}
		if claims.Role == "rider" && assignedRider != claims.UserID {
			return errors.New("forbidden: trip is not assigned to this rider")
		}
		if err := ValidateTripTransition(curStatus, in.NextStatus); err != nil {
			return err
		}

		_, err := tx.Exec(r.Context(), `
			UPDATE delivery_trips
			SET status = $2,
			    pod_photo_url = COALESCE(NULLIF($3, ''), pod_photo_url),
			    cod_collected_amount = CASE WHEN $2 = 'delivered' THEN $4::numeric ELSE cod_collected_amount END,
			    delivered_at = CASE WHEN $2 = 'delivered' THEN NOW() ELSE delivered_at END
			WHERE id = $1
		`, tripID, in.NextStatus, in.PODPhotoURL, in.CODCollectedAmount.StringFixed(4))
		if err != nil {
			return err
		}

		if in.ClientEventID != "" {
			_, err = tx.Exec(r.Context(), `
				INSERT INTO processed_commands (command_id, tenant_id, branch_id, command_type, result_status)
				VALUES ($1, $2, $3, 'delivery.status_updated', 'acked')
				ON CONFLICT (command_id) DO NOTHING
			`, in.ClientEventID, claims.TenantID, in.BranchID)
		}
		return err
	})
	if err != nil {
		httpx.WriteError(w, http.StatusConflict, "TRIP_STATUS_UPDATE_FAILED", err.Error(), nil)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{
		"trip_id":   tripID,
		"status":    in.NextStatus,
		"duplicate": duplicate,
	}, nil)
}

func (h *Handler) ReconcileCOD(w http.ResponseWriter, r *http.Request) {
	var in struct {
		ExpectedAmounts  []decimal.Decimal `json:"expected_amounts"`
		CollectedAmounts []decimal.Decimal `json:"collected_amounts"`
		HandedOverCash   decimal.Decimal   `json:"handed_over_cash"`
	}
	if err := httpx.DecodeJSON(r, &in); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "INVALID_PAYLOAD", err.Error(), nil)
		return
	}
	res := ComputeCODReconciliation(in.ExpectedAmounts, in.CollectedAmounts, in.HandedOverCash)
	httpx.WriteJSON(w, http.StatusOK, res, nil)
}
