package commands_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/hospitality-os/branch-service/internal/commands"
	"github.com/hospitality-os/branch-service/internal/sqlite"
)

func TestCashShiftBlindCloseVarianceAndPostPrepRefundAudit(t *testing.T) {
	store, err := sqlite.Open(filepath.Join(t.TempDir(), "shift_audit.db"))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer store.Close()

	proc := commands.NewProcessor(store, nil)
	ctx := context.Background()

	// 1. Open Cash Shift with 5,000.00 float
	shiftID, err := proc.OpenCashShift(ctx, commands.OpenShiftInput{
		ShiftID:      "sh-001",
		OpenedBy:     "usr-cashier-1",
		DeviceID:     "dev-pos-1",
		OpeningFloat: "5000.00",
	})
	if err != nil {
		t.Fatalf("OpenCashShift failed: %v", err)
	}

	// 2. Create an order & record 1,200.00 cash payment tied to this shift
	ordRes, err := proc.CreateOrUpdateOrder(ctx, commands.CreateOrUpdateOrderInput{
		OrderID:        "ord-shift-1",
		TenantID:       "tn-1",
		BranchID:       "br-1",
		OrderNumber:    "F7-501",
		OrderMode:      "quick_sale",
		Subtotal:       "1200.00",
		TaxTotal:       "0.00",
		DiscountTotal:  "0.00",
		GrandTotal:     "1200.00",
		IdempotencyKey: "cmd-ord-shift-1",
		Items: []commands.OrderItemInput{
			{MenuItemID: "i1", ItemName: "Latte", PrepStation: "barista", Quantity: "1", UnitPrice: "1200.00", LineTotal: "1200.00"},
		},
	})
	if err != nil {
		t.Fatalf("CreateOrUpdateOrder failed: %v", err)
	}
	if _, _, err := proc.RecordPayment(ctx, commands.RecordPaymentInput{
		OrderID:        ordRes.OrderID,
		CashShiftID:    shiftID,
		CheckNumber:    1,
		Method:         "cash",
		Amount:         "1200.00",
		TenderedAmount: "1200.00",
		ChangeDue:      "0.00",
	}); err != nil {
		t.Fatalf("RecordPayment failed: %v", err)
	}

	// 3. Record 300.00 Paid Out (milk run) with manager approval
	if _, err := proc.RecordCashMovement(ctx, commands.CashMovementInput{
		ShiftID:      shiftID,
		TenantID:     "tn-1",
		BranchID:     "br-1",
		MovementType: "paid_out",
		Amount:       "300.00",
		Reason:       "Emergency oat milk purchase",
		ActorUserID:  "usr-cashier-1",
		ApprovedBy:   "usr-mgr-1",
	}); err != nil {
		t.Fatalf("RecordCashMovement failed: %v", err)
	}

	// 4. Blind Close with counted 5,850.00 (expected = 5000 + 1200 - 300 = 5900 -> variance = -50.00)
	zRep, err := proc.CloseCashShift(ctx, commands.CloseShiftInput{
		ShiftID:     shiftID,
		ClosedBy:    "usr-mgr-1",
		CountedCash: "5850.00",
	})
	if err != nil {
		t.Fatalf("CloseCashShift failed: %v", err)
	}
	if zRep.ExpectedCash != "5900.00" || zRep.Variance != "-50.00" {
		t.Fatalf("unexpected Z-report math: expected=5900.00 variance=-50.00, got %+v", zRep)
	}

	// 5. Post-prep refund writes immutable audit log
	auditID, err := proc.VoidOrRefundOrder(ctx, commands.VoidRefundInput{
		TenantID:       "tn-1",
		BranchID:       "br-1",
		OrderID:        ordRes.OrderID,
		ActionType:     "refund",
		Amount:         "1200.00",
		RecordWaste:    true,
		ReasonCode:     "QUALITY_ISSUE",
		ActorUserID:    "usr-cashier-1",
		ApproverUserID: "usr-mgr-1",
	})
	if err != nil || auditID == "" {
		t.Fatalf("VoidOrRefundOrder failed: %v", err)
	}

	var auditCount int
	_ = store.DB().QueryRow(`SELECT COUNT(*) FROM local_audit_logs WHERE id = ?`, auditID).Scan(&auditCount)
	if auditCount != 1 {
		t.Fatalf("expected 1 immutable audit log row, got %d", auditCount)
	}
}
