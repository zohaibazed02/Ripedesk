package hr

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"net/http"

	"github.com/hospitality-os/cloud-api/internal/platform/httpx"
	"github.com/shopspring/decimal"
)

type AttendanceRecord struct {
	EmployeeCode  string
	StaffName     string
	Role          string
	BranchCode    string
	Status        string // pending_review | approved | rejected
	RegularHours  decimal.Decimal
	OvertimeHours decimal.Decimal
	LateIncident  bool
}

type EmployeePayrollSummary struct {
	EmployeeCode  string          `json:"employee_code"`
	StaffName     string          `json:"staff_name"`
	Role          string          `json:"role"`
	BranchCode    string          `json:"branch_code"`
	ShiftsWorked  int             `json:"shifts_worked"`
	RegularHours  decimal.Decimal `json:"regular_hours"`
	OvertimeHours decimal.Decimal `json:"overtime_hours"`
	TotalHours    decimal.Decimal `json:"total_approved_hours"`
	LateIncidents int             `json:"late_incidents"`
}

type ApprovedHoursExport struct {
	Employees       []EmployeePayrollSummary `json:"employees"`
	UnapprovedCount int                      `json:"unapproved_count"`
}

// BuildApprovedHoursExport strictly aggregates ONLY approved attendance records
// and surfaces unapproved_count for any pending/exception logs in the period.
func BuildApprovedHoursExport(records []AttendanceRecord) ApprovedHoursExport {
	byEmp := map[string]*EmployeePayrollSummary{}
	order := []string{}
	unapproved := 0

	for _, rec := range records {
		if rec.Status != "approved" {
			unapproved++
			continue
		}
		emp, ok := byEmp[rec.EmployeeCode]
		if !ok {
			emp = &EmployeePayrollSummary{
				EmployeeCode: rec.EmployeeCode,
				StaffName:    rec.StaffName,
				Role:         rec.Role,
				BranchCode:   rec.BranchCode,
			}
			byEmp[rec.EmployeeCode] = emp
			order = append(order, rec.EmployeeCode)
		}
		emp.ShiftsWorked++
		emp.RegularHours = emp.RegularHours.Add(rec.RegularHours).Round(2)
		emp.OvertimeHours = emp.OvertimeHours.Add(rec.OvertimeHours).Round(2)
		emp.TotalHours = emp.RegularHours.Add(emp.OvertimeHours).Round(2)
		if rec.LateIncident {
			emp.LateIncidents++
		}
	}

	list := make([]EmployeePayrollSummary, 0, len(order))
	for _, code := range order {
		list = append(list, *byEmp[code])
	}
	return ApprovedHoursExport{
		Employees:       list,
		UnapprovedCount: unapproved,
	}
}

func RenderApprovedHoursCSV(exp ApprovedHoursExport) string {
	var buf bytes.Buffer
	w := csv.NewWriter(&buf)
	_ = w.Write([]string{
		"employee_code", "staff_name", "role", "branch_code",
		"shifts_worked", "regular_hours", "overtime_hours", "total_approved_hours", "late_incidents",
	})
	for _, e := range exp.Employees {
		_ = w.Write([]string{
			e.EmployeeCode,
			e.StaffName,
			e.Role,
			e.BranchCode,
			fmt.Sprintf("%d", e.ShiftsWorked),
			e.RegularHours.StringFixed(2),
			e.OvertimeHours.StringFixed(2),
			e.TotalHours.StringFixed(2),
			fmt.Sprintf("%d", e.LateIncidents),
		})
	}
	w.Flush()
	return buf.String()
}

func (h *Handler) ExportApprovedHours(w http.ResponseWriter, r *http.Request) {
	exp := BuildApprovedHoursExport(nil)
	if r.URL.Query().Get("format") == "csv" {
		w.Header().Set("Content-Type", "text/csv; charset=utf-8")
		w.Header().Set("Content-Disposition", `attachment; filename="approved-hours.csv"`)
		w.Header().Set("X-Unapproved-Count", fmt.Sprintf("%d", exp.UnapprovedCount))
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(RenderApprovedHoursCSV(exp)))
		return
	}
	httpx.WriteJSON(w, http.StatusOK, exp, map[string]int{"unapproved_count": exp.UnapprovedCount})
}
