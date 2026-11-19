package hr

import (
	"net/http"
	"time"

	"github.com/hospitality-os/cloud-api/internal/platform/httpx"
	"github.com/shopspring/decimal"
)

type AttendanceEvalInput struct {
	ClockInAt    time.Time
	ClockOutAt   *time.Time
	BreakMinutes int
}

type AttendanceEvalResult struct {
	RegularHours  decimal.Decimal `json:"regular_hours"`
	OvertimeHours decimal.Decimal `json:"overtime_hours"`
	TotalHours    decimal.Decimal `json:"total_hours"`
	LateMinutes   int             `json:"late_minutes"`
	Exceptions    []string        `json:"exceptions"`
}

// EvaluateAttendanceExceptions detects late clock-in, early clock-out, missing clock-out,
// unscheduled shifts, and exact shopspring/decimal regular vs overtime hours split.
func EvaluateAttendanceExceptions(
	in AttendanceEvalInput,
	scheduled *RosterShift,
	graceMinutes int,
	regularThresholdHours decimal.Decimal,
	now time.Time,
) AttendanceEvalResult {
	if graceMinutes <= 0 {
		graceMinutes = 10
	}
	if regularThresholdHours.LessThanOrEqual(decimal.Zero) {
		regularThresholdHours = decimal.NewFromInt(8)
	}

	var exceptions []string
	lateMins := 0

	if scheduled == nil {
		exceptions = append(exceptions, "unscheduled_shift")
	} else {
		graceDur := time.Duration(graceMinutes) * time.Minute
		if in.ClockInAt.After(scheduled.ShiftStart.Add(graceDur)) {
			lateMins = int(in.ClockInAt.Sub(scheduled.ShiftStart).Minutes())
			exceptions = append(exceptions, "late_clock_in")
		}
		if in.ClockOutAt == nil && now.After(scheduled.ShiftEnd.Add(4*time.Hour)) {
			exceptions = append(exceptions, "missing_clock_out")
		}
		if in.ClockOutAt != nil && in.ClockOutAt.Before(scheduled.ShiftEnd.Add(-graceDur)) {
			exceptions = append(exceptions, "early_clock_out")
		}
	}

	if in.ClockOutAt == nil {
		return AttendanceEvalResult{
			RegularHours:  decimal.Zero,
			OvertimeHours: decimal.Zero,
			TotalHours:    decimal.Zero,
			LateMinutes:   lateMins,
			Exceptions:    exceptions,
		}
	}

	netMinutes := int64(in.ClockOutAt.Sub(in.ClockInAt).Minutes()) - int64(in.BreakMinutes)
	if netMinutes < 0 {
		netMinutes = 0
	}
	totalHours := decimal.NewFromInt(netMinutes).Div(decimal.NewFromInt(60)).Round(2)

	regular := totalHours
	overtime := decimal.Zero
	if totalHours.GreaterThan(regularThresholdHours) {
		regular = regularThresholdHours.Round(2)
		overtime = totalHours.Sub(regular).Round(2)
		exceptions = append(exceptions, "overtime_exceeded")
	}

	return AttendanceEvalResult{
		RegularHours:  regular,
		OvertimeHours: overtime,
		TotalHours:    totalHours,
		LateMinutes:   lateMins,
		Exceptions:    exceptions,
	}
}

func (h *Handler) RecordPINPunch(w http.ResponseWriter, r *http.Request) {
	var in struct {
		BranchID       string `json:"branch_id"`
		PIN            string `json:"pin"`
		PunchType      string `json:"punch_type"`
		IdempotencyKey string `json:"idempotency_key"`
	}
	if err := httpx.DecodeJSON(r, &in); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "INVALID_PAYLOAD", err.Error(), nil)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, map[string]any{
		"status":          "recorded",
		"punch_type":      in.PunchType,
		"idempotency_key": in.IdempotencyKey,
		"recorded_at":     time.Now().UTC().Format(time.RFC3339),
	}, nil)
}

func (h *Handler) ApproveAttendance(w http.ResponseWriter, r *http.Request) {
	logID := r.PathValue("id")
	var in struct {
		AdjustmentReason string `json:"adjustment_reason"`
	}
	_ = httpx.DecodeJSON(r, &in)
	httpx.WriteJSON(w, http.StatusOK, map[string]string{
		"id":                logID,
		"status":            "approved",
		"adjustment_reason": in.AdjustmentReason,
	}, nil)
}
