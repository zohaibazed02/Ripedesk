package commands

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/hospitality-os/branch-service/internal/hardware/escpos"
	"github.com/hospitality-os/branch-service/internal/hardware/spooler"
	"github.com/hospitality-os/branch-service/internal/sqlite"
)

var (
	ErrVersionConflict = errors.New("optimistic concurrency conflict: expected_version mismatch")
	ErrNotFound        = errors.New("resource not found")
)

type LANEvent struct {
	SeqID       int64  `json:"seq_id"`
	EventType   string `json:"event_type"`
	AggregateID string `json:"aggregate_id"`
	PayloadJSON string `json:"payload_json"`
	CreatedAt   string `json:"created_at"`
}

type EventPublisher interface {
	PublishEvent(ev LANEvent)
}

type Processor struct {
	store     *sqlite.Store
	publisher EventPublisher
	spooler   *spooler.Spooler
}

func NewProcessor(store *sqlite.Store, publisher EventPublisher) *Processor {
	return &Processor{store: store, publisher: publisher}
}

func (p *Processor) SetSpooler(sp *spooler.Spooler) {
	p.spooler = sp
}

type OrderItemModifierInput struct {
	ModifierID   string `json:"modifier_id"`
	ModifierName string `json:"modifier_name"`
	Quantity     string `json:"quantity"`
	PriceDelta   string `json:"price_delta"`
}

type OrderItemInput struct {
	MenuItemID  string                   `json:"menu_item_id"`
	ItemName    string                   `json:"item_name"`
	PrepStation string                   `json:"prep_station"`
	Quantity    string                   `json:"quantity"`
	UnitPrice   string                   `json:"unit_price"`
	LineTotal   string                   `json:"line_total"`
	SeatNumber  *int                     `json:"seat_number,omitempty"`
	CheckNumber int                      `json:"check_number"`
	Notes       string                   `json:"notes,omitempty"`
	Modifiers   []OrderItemModifierInput `json:"modifiers,omitempty"`
}

type CreateOrUpdateOrderInput struct {
	OrderID         string           `json:"order_id"`
	TenantID        string           `json:"tenant_id"`
	BranchID        string           `json:"branch_id"`
	OrderNumber     string           `json:"order_number"`
	OrderMode       string           `json:"order_mode"`
	TableID         string           `json:"table_id,omitempty"`
	ExpectedVersion int              `json:"expected_version"`
	Subtotal        string           `json:"subtotal"`
	TaxTotal        string           `json:"tax_total"`
	DiscountTotal   string           `json:"discount_total"`
	GrandTotal      string           `json:"grand_total"`
	DeviceID        string           `json:"device_id"`
	IdempotencyKey  string           `json:"idempotency_key"`
	Items           []OrderItemInput `json:"items"`
}

type OrderCommandResult struct {
	OrderID       string   `json:"order_id"`
	OrderVersion  int      `json:"order_version"`
	RoundNumber   int      `json:"round_number"`
	TicketIDs     []string `json:"ticket_ids"`
	LANSeqID      int64    `json:"lan_seq_id"`
	OutboxEventID string   `json:"outbox_event_id"`
}

func (p *Processor) CreateOrUpdateOrder(ctx context.Context, in CreateOrUpdateOrderInput) (*OrderCommandResult, error) {
	if in.OrderID == "" {
		in.OrderID = uuid.NewString()
	}
	if in.IdempotencyKey == "" {
		in.IdempotencyKey = "ord-" + uuid.NewString()
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)

	conn, err := p.store.BeginImmediate(ctx)
	if err != nil {
		return nil, err
	}
	defer func() {
		_, _ = conn.ExecContext(context.Background(), "ROLLBACK")
		_ = conn.Close()
	}()

	// 1. Check existing order version for optimistic concurrency
	var currentVersion int
	err = conn.QueryRowContext(ctx, `SELECT order_version FROM local_orders WHERE id = ?`, in.OrderID).Scan(&currentVersion)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	if errors.Is(err, sql.ErrNoRows) {
		if in.ExpectedVersion != 0 {
			return nil, ErrVersionConflict
		}
		currentVersion = 0
	} else if currentVersion != in.ExpectedVersion {
		return nil, ErrVersionConflict
	}

	nextVersion := currentVersion + 1

	// 2. Upsert local_orders
	var tableID any = nil
	if in.TableID != "" {
		tableID = in.TableID
	}
	if currentVersion == 0 {
		_, err = conn.ExecContext(ctx, `
			INSERT INTO local_orders (
				id, tenant_id, branch_id, order_number, order_mode, table_id,
				order_version, status, subtotal, tax_total, discount_total, grand_total,
				device_id, created_offline_at, updated_at
			) VALUES (?, ?, ?, ?, ?, ?, ?, 'in_prep', ?, ?, ?, ?, ?, ?, ?)
		`, in.OrderID, in.TenantID, in.BranchID, in.OrderNumber, in.OrderMode, tableID,
			nextVersion, in.Subtotal, in.TaxTotal, in.DiscountTotal, in.GrandTotal,
			in.DeviceID, now, now)
	} else {
		_, err = conn.ExecContext(ctx, `
			UPDATE local_orders
			SET order_version = ?, status = 'in_prep', subtotal = ?, tax_total = ?,
			    discount_total = ?, grand_total = ?, updated_at = ?
			WHERE id = ?
		`, nextVersion, in.Subtotal, in.TaxTotal, in.DiscountTotal, in.GrandTotal, now, in.OrderID)
	}
	if err != nil {
		return nil, fmt.Errorf("upsert local_orders: %w", err)
	}

	// 3. Update local_tables if dine-in table is attached
	if in.TableID != "" {
		_, err = conn.ExecContext(ctx, `
			UPDATE local_tables
			SET status = 'occupied', active_order_id = ?, order_version = ?, updated_at = ?
			WHERE id = ?
		`, in.OrderID, nextVersion, now, in.TableID)
		if err != nil {
			return nil, fmt.Errorf("update local_tables: %w", err)
		}
	}

	// 4. Create new order round
	var maxRound int
	_ = conn.QueryRowContext(ctx, `SELECT COALESCE(MAX(round_number), 0) FROM local_order_rounds WHERE order_id = ?`, in.OrderID).Scan(&maxRound)
	roundNumber := maxRound + 1
	roundID := uuid.NewString()
	if _, err := conn.ExecContext(ctx, `
		INSERT INTO local_order_rounds (id, order_id, round_number, created_at)
		VALUES (?, ?, ?, ?)
	`, roundID, in.OrderID, roundNumber, now); err != nil {
		return nil, fmt.Errorf("insert local_order_rounds: %w", err)
	}

	// 5. Insert order items, modifiers, and group by prep_station for KDS tickets
	type stationItem struct {
		orderItemID string
		itemName    string
		quantity    string
	}
	byStation := map[string][]stationItem{}

	for _, item := range in.Items {
		itemID := uuid.NewString()
		checkNum := item.CheckNumber
		if checkNum <= 0 {
			checkNum = 1
		}
		station := item.PrepStation
		if station == "" {
			station = "hot_kitchen"
		}
		if _, err := conn.ExecContext(ctx, `
			INSERT INTO local_order_items (
				id, order_id, order_round_id, menu_item_id, item_name, prep_station,
				quantity, unit_price, line_total, seat_number, check_number, notes
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`, itemID, in.OrderID, roundID, item.MenuItemID, item.ItemName, station,
			item.Quantity, item.UnitPrice, item.LineTotal, item.SeatNumber, checkNum, item.Notes); err != nil {
			return nil, fmt.Errorf("insert local_order_items: %w", err)
		}

		for _, mod := range item.Modifiers {
			qty := mod.Quantity
			if qty == "" {
				qty = "1"
			}
			if _, err := conn.ExecContext(ctx, `
				INSERT INTO local_order_item_modifiers (id, order_item_id, modifier_id, modifier_name, quantity, price_delta)
				VALUES (?, ?, ?, ?, ?, ?)
			`, uuid.NewString(), itemID, mod.ModifierID, mod.ModifierName, qty, mod.PriceDelta); err != nil {
				return nil, fmt.Errorf("insert local_order_item_modifiers: %w", err)
			}
		}

		byStation[station] = append(byStation[station], stationItem{
			orderItemID: itemID,
			itemName:    item.ItemName,
			quantity:    item.Quantity,
		})
	}

	var ticketIDs []string
	for station, sItems := range byStation {
		ticketID := uuid.NewString()
		if _, err := conn.ExecContext(ctx, `
			INSERT INTO local_kitchen_tickets (id, order_id, order_round_id, prep_station, status, created_at, updated_at)
			VALUES (?, ?, ?, ?, 'queued', ?, ?)
		`, ticketID, in.OrderID, roundID, station, now, now); err != nil {
			return nil, fmt.Errorf("insert local_kitchen_tickets: %w", err)
		}
		for _, si := range sItems {
			if _, err := conn.ExecContext(ctx, `
				INSERT INTO local_kitchen_ticket_items (id, kitchen_ticket_id, order_item_id, item_name, quantity, status)
				VALUES (?, ?, ?, ?, ?, 'queued')
			`, uuid.NewString(), ticketID, si.orderItemID, si.itemName, si.quantity); err != nil {
				return nil, fmt.Errorf("insert local_kitchen_ticket_items: %w", err)
			}
		}
		ticketIDs = append(ticketIDs, ticketID)
	}

	// 6. Write LAN event + Outbox row inside the SAME transaction
	payloadBytes, _ := json.Marshal(map[string]any{
		"order_id":      in.OrderID,
		"order_number":  in.OrderNumber,
		"order_mode":    in.OrderMode,
		"order_version": nextVersion,
		"round_number":  roundNumber,
		"ticket_ids":    ticketIDs,
		"grand_total":   in.GrandTotal,
	})
	payloadStr := string(payloadBytes)

	res, err := conn.ExecContext(ctx, `
		INSERT INTO lan_events (event_type, aggregate_id, payload_json, created_at)
		VALUES ('order.upserted', ?, ?, ?)
	`, in.OrderID, payloadStr, now)
	if err != nil {
		return nil, fmt.Errorf("insert lan_events: %w", err)
	}
	lanSeqID, _ := res.LastInsertId()

	eventID := uuid.NewString()
	if _, err := conn.ExecContext(ctx, `
		INSERT INTO local_outbox (event_id, command_type, aggregate_type, aggregate_id, payload_json, idempotency_key, status, created_at)
		VALUES (?, 'order.upserted', 'order', ?, ?, ?, 'pending', ?)
	`, eventID, in.OrderID, payloadStr, in.IdempotencyKey, now); err != nil {
		return nil, fmt.Errorf("insert local_outbox: %w", err)
	}

	if _, err := conn.ExecContext(ctx, "COMMIT"); err != nil {
		return nil, fmt.Errorf("commit order tx: %w", err)
	}
	// Release the single SQLite connection back to the pool BEFORE calling spooler (prevents MaxOpenConns(1) self-deadlock)
	_ = conn.Close()

	if p.publisher != nil {
		p.publisher.PublishEvent(LANEvent{
			SeqID:       lanSeqID,
			EventType:   "order.upserted",
			AggregateID: in.OrderID,
			PayloadJSON: payloadStr,
			CreatedAt:   now,
		})
	}

	// Auto-enqueue station kitchen tickets to durable print spooler if configured
	if p.spooler != nil {
		for station, sItems := range byStation {
			var kItems []escpos.KitchenTicketItem
			for _, si := range sItems {
				kItems = append(kItems, escpos.KitchenTicketItem{
					Quantity: si.quantity,
					Name:     si.itemName,
				})
			}
			frame := escpos.RenderKitchenTicket(escpos.KitchenTicketData{
				PaperWidth:  escpos.Paper80mm,
				StationName: station,
				OrderMode:   in.OrderMode,
				OrderNumber: in.OrderNumber,
				RoundNumber: roundNumber,
				Timestamp:   now,
				Items:       kItems,
			})
			_, _ = p.spooler.EnqueueForStation(ctx, station, "kitchen_ticket", fmt.Sprintf("kt-%s-r%d-%s", in.OrderID, roundNumber, station), frame)
		}
	}

	return &OrderCommandResult{
		OrderID:       in.OrderID,
		OrderVersion:  nextVersion,
		RoundNumber:   roundNumber,
		TicketIDs:     ticketIDs,
		LANSeqID:      lanSeqID,
		OutboxEventID: eventID,
	}, nil
}

type BumpTicketInput struct {
	TicketID       string `json:"ticket_id"`
	Status         string `json:"status"` // preparing, bumped, recalled
	IdempotencyKey string `json:"idempotency_key"`
}

func (p *Processor) BumpOrRecallTicket(ctx context.Context, in BumpTicketInput) (int64, error) {
	if in.IdempotencyKey == "" {
		in.IdempotencyKey = "kds-" + uuid.NewString()
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)

	conn, err := p.store.BeginImmediate(ctx)
	if err != nil {
		return 0, err
	}
	defer func() {
		_, _ = conn.ExecContext(context.Background(), "ROLLBACK")
		_ = conn.Close()
	}()

	var orderID, roundID string
	if err := conn.QueryRowContext(ctx, `
		SELECT order_id, order_round_id FROM local_kitchen_tickets WHERE id = ?
	`, in.TicketID).Scan(&orderID, &roundID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, ErrNotFound
		}
		return 0, err
	}

	var bumpedAt, recalledAt any
	if in.Status == "bumped" {
		bumpedAt = now
	} else if in.Status == "recalled" {
		recalledAt = now
	}

	if _, err := conn.ExecContext(ctx, `
		UPDATE local_kitchen_tickets
		SET status = ?, bumped_at = COALESCE(?, bumped_at), recalled_at = COALESCE(?, recalled_at), updated_at = ?
		WHERE id = ?
	`, in.Status, bumpedAt, recalledAt, now, in.TicketID); err != nil {
		return 0, err
	}
	if _, err := conn.ExecContext(ctx, `
		UPDATE local_kitchen_ticket_items SET status = ? WHERE kitchen_ticket_id = ?
	`, in.Status, in.TicketID); err != nil {
		return 0, err
	}
	if in.Status == "preparing" || in.Status == "bumped" {
		_, _ = conn.ExecContext(ctx, `
			UPDATE local_order_rounds SET prep_confirmed_at = COALESCE(prep_confirmed_at, ?) WHERE id = ?
		`, now, roundID)
	}

	payloadBytes, _ := json.Marshal(map[string]any{
		"ticket_id": in.TicketID,
		"order_id":  orderID,
		"status":    in.Status,
	})
	payloadStr := string(payloadBytes)

	res, err := conn.ExecContext(ctx, `
		INSERT INTO lan_events (event_type, aggregate_id, payload_json, created_at)
		VALUES ('kds.ticket_updated', ?, ?, ?)
	`, in.TicketID, payloadStr, now)
	if err != nil {
		return 0, err
	}
	lanSeqID, _ := res.LastInsertId()

	if _, err := conn.ExecContext(ctx, `
		INSERT INTO local_outbox (event_id, command_type, aggregate_type, aggregate_id, payload_json, idempotency_key, status, created_at)
		VALUES (?, 'kds.ticket_updated', 'kitchen_ticket', ?, ?, ?, 'pending', ?)
	`, uuid.NewString(), in.TicketID, payloadStr, in.IdempotencyKey, now); err != nil {
		return 0, err
	}

	if _, err := conn.ExecContext(ctx, "COMMIT"); err != nil {
		return 0, err
	}

	if p.publisher != nil {
		p.publisher.PublishEvent(LANEvent{
			SeqID:       lanSeqID,
			EventType:   "kds.ticket_updated",
			AggregateID: in.TicketID,
			PayloadJSON: payloadStr,
			CreatedAt:   now,
		})
	}
	return lanSeqID, nil
}

type RecordPaymentInput struct {
	PaymentID      string `json:"payment_id"`
	OrderID        string `json:"order_id"`
	CashShiftID    string `json:"cash_shift_id,omitempty"`
	CheckNumber    int    `json:"check_number"`
	Method         string `json:"method"`
	Amount         string `json:"amount"`
	TenderedAmount string `json:"tendered_amount"`
	ChangeDue      string `json:"change_due"`
	ProviderRef    string `json:"provider_ref,omitempty"`
	IdempotencyKey string `json:"idempotency_key"`
}

func (p *Processor) RecordPayment(ctx context.Context, in RecordPaymentInput) (int64, bool, error) {
	if in.PaymentID == "" {
		in.PaymentID = uuid.NewString()
	}
	if in.CheckNumber <= 0 {
		in.CheckNumber = 1
	}
	if in.IdempotencyKey == "" {
		in.IdempotencyKey = "pay-" + in.PaymentID
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)

	conn, err := p.store.BeginImmediate(ctx)
	if err != nil {
		return 0, false, err
	}
	defer func() {
		_, _ = conn.ExecContext(context.Background(), "ROLLBACK")
		_ = conn.Close()
	}()

	var grandTotalStr string
	var tableID sql.NullString
	if err := conn.QueryRowContext(ctx, `
		SELECT grand_total, table_id FROM local_orders WHERE id = ?
	`, in.OrderID).Scan(&grandTotalStr, &tableID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, false, ErrNotFound
		}
		return 0, false, err
	}

	if _, err := conn.ExecContext(ctx, `
		INSERT INTO local_payments (
			id, order_id, cash_shift_id, check_number, method, status,
			amount, tendered_amount, change_due, provider_ref, created_at
		) VALUES (?, ?, NULLIF(?, ''), ?, ?, 'captured', ?, ?, ?, ?, ?)
	`, in.PaymentID, in.OrderID, in.CashShiftID, in.CheckNumber, in.Method,
		in.Amount, in.TenderedAmount, in.ChangeDue, in.ProviderRef, now); err != nil {
		return 0, false, err
	}

	// Sum captured payments for settlement check
	rows, err := conn.QueryContext(ctx, `
		SELECT amount FROM local_payments WHERE order_id = ? AND status = 'captured'
	`, in.OrderID)
	if err != nil {
		return 0, false, err
	}
	var paidTotal float64
	for rows.Next() {
		var amtStr string
		if err := rows.Scan(&amtStr); err == nil {
			if v, err := strconv.ParseFloat(amtStr, 64); err == nil {
				paidTotal += v
			}
		}
	}
	rows.Close()

	grandTotal, _ := strconv.ParseFloat(grandTotalStr, 64)
	settled := paidTotal+0.0001 >= grandTotal

	if settled {
		_, _ = conn.ExecContext(ctx, `UPDATE local_orders SET status = 'completed', updated_at = ? WHERE id = ?`, now, in.OrderID)
		if tableID.Valid && tableID.String != "" {
			_, _ = conn.ExecContext(ctx, `
				UPDATE local_tables SET status = 'dirty', active_order_id = NULL, updated_at = ? WHERE id = ?
			`, now, tableID.String)
		}
	}

	payloadBytes, _ := json.Marshal(map[string]any{
		"payment_id":   in.PaymentID,
		"order_id":     in.OrderID,
		"check_number": in.CheckNumber,
		"method":       in.Method,
		"amount":       in.Amount,
		"settled":      settled,
	})
	payloadStr := string(payloadBytes)

	res, err := conn.ExecContext(ctx, `
		INSERT INTO lan_events (event_type, aggregate_id, payload_json, created_at)
		VALUES ('payment.recorded', ?, ?, ?)
	`, in.OrderID, payloadStr, now)
	if err != nil {
		return 0, false, err
	}
	lanSeqID, _ := res.LastInsertId()

	if _, err := conn.ExecContext(ctx, `
		INSERT INTO local_outbox (event_id, command_type, aggregate_type, aggregate_id, payload_json, idempotency_key, status, created_at)
		VALUES (?, 'payment.recorded', 'payment', ?, ?, ?, 'pending', ?)
	`, uuid.NewString(), in.PaymentID, payloadStr, in.IdempotencyKey, now); err != nil {
		return 0, false, err
	}

	if _, err := conn.ExecContext(ctx, "COMMIT"); err != nil {
		return 0, false, err
	}

	if p.publisher != nil {
		p.publisher.PublishEvent(LANEvent{
			SeqID:       lanSeqID,
			EventType:   "payment.recorded",
			AggregateID: in.OrderID,
			PayloadJSON: payloadStr,
			CreatedAt:   now,
		})
	}

	// Auto-enqueue receipt + cash drawer kick for cash tenders
	if p.spooler != nil {
		frame := escpos.RenderCustomerReceipt(escpos.ReceiptData{
			PaperWidth:    escpos.Paper80mm,
			BranchName:    "Hospitality OS Branch",
			OrderMode:     "pos",
			OrderNumber:   in.OrderID,
			CheckNumber:   in.CheckNumber,
			Timestamp:     now,
			GrandTotal:    grandTotalStr,
			PaymentMethod: in.Method,
			Tendered:      in.TenderedAmount,
			ChangeDue:     in.ChangeDue,
			OpenDrawer:    in.Method == "cash",
		})
		_, _ = p.spooler.EnqueueForStation(ctx, "receipt_counter", "customer_receipt", "rcpt-"+in.PaymentID, frame)
	}
	return lanSeqID, settled, nil
}
