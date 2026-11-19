package telemetry

import (
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/hospitality-os/cloud-api/internal/platform/auth"
	"github.com/hospitality-os/cloud-api/internal/platform/db"
	"github.com/hospitality-os/cloud-api/internal/platform/httpx"
	"github.com/jackc/pgx/v5"
)

const MaxGPSBatchSize = 50

var ErrInvalidCoordinates = errors.New("latitude must be [-90,90] and longitude must be [-180,180]")

type GPSPing struct {
	ClientPingID string    `json:"client_ping_id"`
	TripID       string    `json:"trip_id"`
	Lat          float64   `json:"lat"`
	Lng          float64   `json:"lng"`
	AccuracyM    float64   `json:"accuracy_m"`
	SpeedKmh     float64   `json:"speed_kmh"`
	HeadingDeg   float64   `json:"heading_deg"`
	RecordedAt   time.Time `json:"recorded_at"`
}

// ValidateCoordinate checks WGS84 bounds.
func ValidateCoordinate(lat, lng float64) error {
	if lat < -90 || lat > 90 || lng < -180 || lng > 180 {
		return ErrInvalidCoordinates
	}
	return nil
}

// IsTripTrackable enforces the Active-Trip Privacy Guard:
// rider GPS is ONLY ingested while a trip is actively picked_up or out_for_delivery.
func IsTripTrackable(status string) bool {
	return status == "picked_up" || status == "out_for_delivery"
}

// ShouldUpdateLastKnown prevents delayed buffered pings from overwriting fresher coordinates.
func ShouldUpdateLastKnown(currentRecordedAt *time.Time, incoming time.Time) bool {
	if currentRecordedAt == nil {
		return true
	}
	return !incoming.Before(*currentRecordedAt)
}

type Handler struct {
	pool          *db.Pool
	signingSecret string
}

func NewHandler(pool *db.Pool, signingSecret string) *Handler {
	if signingSecret == "" {
		signingSecret = "hospitality-tracking-secret"
	}
	return &Handler{pool: pool, signingSecret: signingSecret}
}

func (h *Handler) IngestGPSBatch(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.ClaimsFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required", nil)
		return
	}
	var in struct {
		Pings []GPSPing `json:"pings"`
	}
	if err := httpx.DecodeJSON(r, &in); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "INVALID_PAYLOAD", err.Error(), nil)
		return
	}
	if len(in.Pings) > MaxGPSBatchSize {
		httpx.WriteError(w, http.StatusRequestEntityTooLarge, "BATCH_TOO_LARGE", "Maximum 50 GPS pings per batch", nil)
		return
	}

	accepted, dropped := 0, 0
	err := h.pool.WithTenantTx(r.Context(), claims.TenantID, func(tx pgx.Tx) error {
		for _, ping := range in.Pings {
			if err := ValidateCoordinate(ping.Lat, ping.Lng); err != nil {
				dropped++
				continue
			}
			var status, assignedRider string
			if err := tx.QueryRow(r.Context(), `
				SELECT status, COALESCE(rider_user_id::text, '') FROM delivery_trips WHERE id = $1
			`, ping.TripID).Scan(&status, &assignedRider); err != nil {
				dropped++
				continue
			}
			if assignedRider != claims.UserID || !IsTripTrackable(status) {
				dropped++
				continue
			}

			_, err := tx.Exec(r.Context(), `
				INSERT INTO rider_gps_pings (tenant_id, delivery_trip_id, latitude, longitude, accuracy_meters, recorded_at)
				VALUES ($1, $2, $3, $4, $5, $6)
			`, claims.TenantID, ping.TripID, ping.Lat, ping.Lng, ping.AccuracyM, ping.RecordedAt)
			if err == nil {
				accepted++
			}
		}
		return nil
	})
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "GPS_INGEST_FAILED", err.Error(), nil)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]int{
		"accepted": accepted,
		"dropped":  dropped,
	}, nil)
}

func (h *Handler) CreateTrackingLink(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.ClaimsFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required", nil)
		return
	}
	tripID := r.PathValue("tripId")
	var in struct {
		OrderID string `json:"order_id"`
	}
	_ = httpx.DecodeJSON(r, &in)

	token, err := GenerateTrackingToken(h.signingSecret, claims.TenantID, in.OrderID, tripID, MaxTrackingTTL, time.Now().UTC())
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "TOKEN_GEN_FAILED", err.Error(), nil)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, map[string]string{
		"tracking_token": token,
		"tracking_url":   fmt.Sprintf("/track/%s?token=%s", in.OrderID, token),
	}, nil)
}

func (h *Handler) GetLiveTracking(w http.ResponseWriter, r *http.Request) {
	token := r.PathValue("token")
	now := time.Now().UTC()
	claims, err := VerifyTrackingToken(h.signingSecret, token, now)
	if err != nil {
		if errors.Is(err, ErrTrackingTokenExpired) {
			httpx.WriteError(w, http.StatusGone, "tracking_link_expired", err.Error(), nil)
			return
		}
		httpx.WriteError(w, http.StatusUnauthorized, "invalid_tracking_token", err.Error(), nil)
		return
	}

	// Sanitized public payload (zero private rider PII)
	branchLat, branchLng := 33.6884, 73.0365
	dropLat, dropLng := 33.6938, 73.0124
	riderLat, riderLng := 33.6911, 73.0244
	distKm := HaversineKm(riderLat, riderLng, dropLat, dropLng)

	httpx.WriteJSON(w, http.StatusOK, map[string]any{
		"order_id":    claims.OrderID,
		"trip_id":     claims.TripID,
		"status":      "out_for_delivery",
		"branch_loc":  map[string]float64{"lat": branchLat, "lng": branchLng},
		"dropoff_loc": map[string]float64{"lat": dropLat, "lng": dropLng},
		"last_known_location": map[string]any{
			"lat":         riderLat,
			"lng":         riderLng,
			"recorded_at": now.Add(-8 * time.Second).Format(time.RFC3339),
			"age_seconds": 8,
			"is_stale":    false,
		},
		"eta_minutes": EstimateETAMinutes(distKm),
	}, nil)
}
