package hr_test

import (
	"strings"
	"testing"
	"time"

	"github.com/hospitality-os/cloud-api/internal/hr"
	"github.com/shopspring/decimal"
)

func TestRosterOverlapAttendanceExceptionsAndApprovedExport(t *testing.T) {
	base := time.Date(2026, 11, 20, 9, 0, 0, 0, time.UTC)

	// 1. Roster overlap guard across branches
	existing := []hr.RosterShift{
		{UserID: "u1", BranchID: "br-f7", ShiftStart: base, ShiftEnd: base.Add(8 * time.Hour)},
	}
	overlapping := hr.RosterShift{
		UserID: "u1", BranchID: "br-g9", ShiftStart: base.Add(4 * time.Hour), ShiftEnd: base.Add(10 * time.Hour),
	}
	if !hr.HasRosterOverlap(existing, overlapping) {
		t.Fatalf("expected overlapping shift to be detected")
	}

	// 2. Late clock-in (+20m) & 9.50 net worked hours -> 8.00 regular + 1.50 overtime
	clockIn := base.Add(20 * time.Minute)
	clockOut := clockIn.Add(9*time.Hour + 30*time.Minute)
	ev := hr.EvaluateAttendanceExceptions(
		hr.AttendanceEvalInput{ClockInAt: clockIn, ClockOutAt: &clockOut},
		&existing[0],
		10,
		decimal.NewFromInt(8),
		clockOut,
	)
	if !ev.RegularHours.Equal(decimal.RequireFromString("8.00")) ||
		!ev.OvertimeHours.Equal(decimal.RequireFromString("1.50")) ||
		ev.LateMinutes != 20 {
		t.Fatalf("unexpected attendance eval: %+v", ev)
	}

	// 3. Missing clock-out past shift_end + 4h
	missingEv := hr.EvaluateAttendanceExceptions(
		hr.AttendanceEvalInput{ClockInAt: base, ClockOutAt: nil},
		&existing[0],
		10,
		decimal.NewFromInt(8),
		base.Add(13*time.Hour),
	)
	if len(missingEv.Exceptions) == 0 || missingEv.Exceptions[0] != "missing_clock_out" {
		t.Fatalf("expected missing_clock_out exception, got %+v", missingEv)
	}

	// 4. Approved-hours export excludes unapproved logs and produces valid CSV
	exp := hr.BuildApprovedHoursExport([]hr.AttendanceRecord{
		{EmployeeCode: "EMP-01", StaffName: "Ayesha Khan", Role: "cashier", BranchCode: "ARB-F7", Status: "approved", RegularHours: decimal.RequireFromString("8.00"), OvertimeHours: decimal.RequireFromString("1.50")},
		{EmployeeCode: "EMP-02", StaffName: "Hamza Raza", Role: "waiter", BranchCode: "ARB-G9", Status: "pending_review", RegularHours: decimal.RequireFromString("7.50"), OvertimeHours: decimal.Zero},
	})
	if len(exp.Employees) != 1 || exp.UnapprovedCount != 1 {
		t.Fatalf("expected 1 approved employee and 1 unapproved count, got %+v", exp)
	}
	csvOut := hr.RenderApprovedHoursCSV(exp)
	if !strings.Contains(csvOut, "EMP-01,Ayesha Khan,cashier,ARB-F7,1,8.00,1.50,9.50,0") {
		t.Fatalf("unexpected CSV output:\n%s", csvOut)
	}
}
