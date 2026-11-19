package lan

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/hospitality-os/branch-service/internal/commands"
	"github.com/hospitality-os/branch-service/internal/hardware/escpos"
	"github.com/hospitality-os/branch-service/internal/hardware/spooler"
	"github.com/hospitality-os/branch-service/internal/sqlite"
)

type Server struct {
	addr      string
	store     *sqlite.Store
	hub       *Hub
	processor *commands.Processor
	spooler   *spooler.Spooler
	httpSrv   *http.Server
}

func NewServer(addr string, store *sqlite.Store, hub *Hub, processor *commands.Processor, sp *spooler.Spooler) *Server {
	s := &Server{
		addr:      addr,
		store:     store,
		hub:       hub,
		processor: processor,
		spooler:   sp,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /lan/health", s.handleHealth)
	mux.HandleFunc("GET /lan/catalog", s.handleCatalog)
	mux.HandleFunc("GET /lan/tables", s.handleTables)
	mux.HandleFunc("GET /lan/orders/active", s.handleActiveOrders)
	mux.HandleFunc("GET /lan/kds/tickets", s.handleKDSTickets)
	mux.HandleFunc("POST /lan/commands/orders", s.handleOrderCommand)
	mux.HandleFunc("POST /lan/commands/kds/bump", s.handleKDSBumpCommand)
	mux.HandleFunc("POST /lan/commands/payments", s.handlePaymentCommand)
	mux.HandleFunc("POST /lan/commands/shifts/open", s.handleOpenShift)
	mux.HandleFunc("POST /lan/commands/shifts/movement", s.handleCashMovement)
	mux.HandleFunc("POST /lan/commands/shifts/close", s.handleCloseShift)
	mux.HandleFunc("POST /lan/commands/orders/void-refund", s.handleVoidRefund)
	mux.HandleFunc("POST /lan/commands/hr/pin-punch", s.handleOfflinePINPunch)
	mux.HandleFunc("GET /lan/stream", s.handleSSEStream)
	mux.HandleFunc("GET /lan/events", s.handleEventsCatchup)
	mux.HandleFunc("GET /lan/hardware/printers", s.handleListPrinters)
	mux.HandleFunc("POST /lan/hardware/printers", s.handleRegisterPrinter)
	mux.HandleFunc("GET /lan/hardware/print-jobs", s.handleListPrintJobs)
	mux.HandleFunc("POST /lan/hardware/print-jobs/{id}/reprint", s.handleReprintJob)
	mux.HandleFunc("POST /lan/hardware/cash-drawer/kick", s.handleCashDrawerKick)

	s.httpSrv = &http.Server{
		Addr:              addr,
		Handler:           corsMiddleware(mux),
		ReadHeaderTimeout: 5 * time.Second,
	}
	return s
}

func (s *Server) ListenAndServe() error {
	return s.httpSrv.ListenAndServe()
}

func (s *Server) Shutdown(ctx context.Context) error {
	s.hub.CloseAll()
	return s.httpSrv.Shutdown(ctx)
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Device-ID")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	snap, err := s.store.HealthSnapshot(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, snap)
}

func (s *Server) handleCatalog(w http.ResponseWriter, r *http.Request) {
	var etag, catalogJSON string
	if err := s.store.DB().QueryRowContext(r.Context(), `
		SELECT catalog_etag, catalog_json FROM branch_state WHERE id = 1
	`).Scan(&etag, &catalogJSON); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if etag != "" {
		w.Header().Set("ETag", etag)
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(catalogJSON))
}

func (s *Server) handleTables(w http.ResponseWriter, r *http.Request) {
	rows, err := s.store.DB().QueryContext(r.Context(), `
		SELECT id, floor_id, table_code, capacity, pos_x, pos_y, shape, status, COALESCE(active_order_id, ''), order_version
		FROM local_tables ORDER BY table_code ASC
	`)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	defer rows.Close()

	type tableRow struct {
		ID            string `json:"id"`
		FloorID       string `json:"floor_id"`
		TableCode     string `json:"table_code"`
		Capacity      int    `json:"capacity"`
		PosX          int    `json:"pos_x"`
		PosY          int    `json:"pos_y"`
		Shape         string `json:"shape"`
		Status        string `json:"status"`
		ActiveOrderID string `json:"active_order_id,omitempty"`
		OrderVersion  int    `json:"order_version"`
	}
	var tables []tableRow
	for rows.Next() {
		var t tableRow
		if err := rows.Scan(&t.ID, &t.FloorID, &t.TableCode, &t.Capacity, &t.PosX, &t.PosY, &t.Shape, &t.Status, &t.ActiveOrderID, &t.OrderVersion); err == nil {
			tables = append(tables, t)
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"tables": tables})
}

func (s *Server) handleActiveOrders(w http.ResponseWriter, r *http.Request) {
	rows, err := s.store.DB().QueryContext(r.Context(), `
		SELECT id, order_number, order_mode, COALESCE(table_id, ''), order_version, status, grand_total, created_offline_at
		FROM local_orders
		WHERE status IN ('open', 'in_prep', 'ready')
		ORDER BY created_offline_at DESC
	`)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	defer rows.Close()

	type orderRow struct {
		ID           string `json:"id"`
		OrderNumber  string `json:"order_number"`
		OrderMode    string `json:"order_mode"`
		TableID      string `json:"table_id,omitempty"`
		OrderVersion int    `json:"order_version"`
		Status       string `json:"status"`
		GrandTotal   string `json:"grand_total"`
		CreatedAt    string `json:"created_offline_at"`
	}
	var orders []orderRow
	for rows.Next() {
		var o orderRow
		if err := rows.Scan(&o.ID, &o.OrderNumber, &o.OrderMode, &o.TableID, &o.OrderVersion, &o.Status, &o.GrandTotal, &o.CreatedAt); err == nil {
			orders = append(orders, o)
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"orders": orders})
}

func (s *Server) handleKDSTickets(w http.ResponseWriter, r *http.Request) {
	station := r.URL.Query().Get("station")
	query := `
		SELECT id, order_id, order_round_id, prep_station, status, created_at
		FROM local_kitchen_tickets
		WHERE status IN ('queued', 'preparing', 'recalled')
	`
	var args []any
	if station != "" && station != "all" {
		query += ` AND prep_station = ?`
		args = append(args, station)
	}
	query += ` ORDER BY created_at ASC`

	rows, err := s.store.DB().QueryContext(r.Context(), query, args...)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	defer rows.Close()

	type ticketRow struct {
		ID          string `json:"id"`
		OrderID     string `json:"order_id"`
		RoundID     string `json:"order_round_id"`
		PrepStation string `json:"prep_station"`
		Status      string `json:"status"`
		CreatedAt   string `json:"created_at"`
	}
	var tickets []ticketRow
	for rows.Next() {
		var tk ticketRow
		if err := rows.Scan(&tk.ID, &tk.OrderID, &tk.RoundID, &tk.PrepStation, &tk.Status, &tk.CreatedAt); err == nil {
			tickets = append(tickets, tk)
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"tickets": tickets})
}

func (s *Server) handleOrderCommand(w http.ResponseWriter, r *http.Request) {
	var in commands.CreateOrUpdateOrderInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid order payload"})
		return
	}
	res, err := s.processor.CreateOrUpdateOrder(r.Context(), in)
	if err != nil {
		if errors.Is(err, commands.ErrVersionConflict) {
			writeJSON(w, http.StatusConflict, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func (s *Server) handleKDSBumpCommand(w http.ResponseWriter, r *http.Request) {
	var in commands.BumpTicketInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid kds bump payload"})
		return
	}
	seqID, err := s.processor.BumpOrRecallTicket(r.Context(), in)
	if err != nil {
		if errors.Is(err, commands.ErrNotFound) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "ticket not found"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"lan_seq_id": seqID, "status": in.Status})
}

func (s *Server) handlePaymentCommand(w http.ResponseWriter, r *http.Request) {
	var in commands.RecordPaymentInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid payment payload"})
		return
	}
	seqID, settled, err := s.processor.RecordPayment(r.Context(), in)
	if err != nil {
		if errors.Is(err, commands.ErrNotFound) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "order not found"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"lan_seq_id": seqID, "settled": settled})
}

func (s *Server) handleSSEStream(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "streaming unsupported"})
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	ch := s.hub.Subscribe()
	defer s.hub.Unsubscribe(ch)

	_, _ = fmt.Fprintf(w, "event: connected\ndata: {\"status\":\"ready\"}\n\n")
	flusher.Flush()

	keepAlive := time.NewTicker(20 * time.Second)
	defer keepAlive.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case <-keepAlive.C:
			_, _ = fmt.Fprintf(w, ": heartbeat\n\n")
			flusher.Flush()
		case ev, open := <-ch:
			if !open {
				return
			}
			raw, _ := json.Marshal(ev)
			_, _ = fmt.Fprintf(w, "id: %d\nevent: %s\ndata: %s\n\n", ev.SeqID, ev.EventType, raw)
			flusher.Flush()
		}
	}
}

func (s *Server) handleEventsCatchup(w http.ResponseWriter, r *http.Request) {
	sinceSeq, _ := strconv.ParseInt(r.URL.Query().Get("since_seq"), 10, 64)
	rows, err := s.store.DB().QueryContext(r.Context(), `
		SELECT seq_id, event_type, aggregate_id, payload_json, created_at
		FROM lan_events
		WHERE seq_id > ?
		ORDER BY seq_id ASC
		LIMIT 500
	`, sinceSeq)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	defer rows.Close()

	var events []Event
	for rows.Next() {
		var ev Event
		if err := rows.Scan(&ev.SeqID, &ev.EventType, &ev.AggregateID, &ev.PayloadJSON, &ev.CreatedAt); err == nil {
			events = append(events, ev)
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"events": events})
}

func (s *Server) handleListPrinters(w http.ResponseWriter, r *http.Request) {
	printers, err := s.spooler.ListPrinters(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"printers": printers})
}

func (s *Server) handleRegisterPrinter(w http.ResponseWriter, r *http.Request) {
	var in struct {
		spooler.Printer
		TestPrint bool `json:"test_print"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid printer payload"})
		return
	}
	p, err := s.spooler.RegisterPrinter(r.Context(), in.Printer)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	var testJobID string
	if in.TestPrint {
		frame := escpos.NewEncoder(escpos.Paper80mm).Init().Align(escpos.AlignCenter).Bold(true).
			Line("HOSPITALITY OS TEST PRINT").Bold(false).Line("Printer: "+p.Name).Feed(3).Cut(true).Bytes()
		testJobID, _ = s.spooler.EnqueueJob(r.Context(), p.ID, "test_print", "", frame)
	}
	writeJSON(w, http.StatusCreated, map[string]any{"printer": p, "test_job_id": testJobID})
}

func (s *Server) handleListPrintJobs(w http.ResponseWriter, r *http.Request) {
	status := r.URL.Query().Get("status")
	jobs, err := s.spooler.ListJobs(r.Context(), status)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"jobs": jobs})
}

func (s *Server) handleReprintJob(w http.ResponseWriter, r *http.Request) {
	jobID := r.PathValue("id")
	if err := s.spooler.ReprintJob(r.Context(), jobID); err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"job_id": jobID, "status": "queued"})
}

func (s *Server) handleCashDrawerKick(w http.ResponseWriter, r *http.Request) {
	var in struct {
		ManagerOverridePIN string `json:"manager_override_pin"`
		PaymentID          string `json:"payment_id"`
		Reason             string `json:"reason"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid cash drawer kick payload"})
		return
	}
	if in.ManagerOverridePIN == "" && in.PaymentID == "" {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "manager_override_pin or active payment_id is required to open cash drawer"})
		return
	}
	jobID, err := s.spooler.KickCashDrawer(r.Context(), "")
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"job_id": jobID, "status": "queued"})
}

func (s *Server) handleOpenShift(w http.ResponseWriter, r *http.Request) {
	var in commands.OpenShiftInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid open shift payload"})
		return
	}
	id, err := s.processor.OpenCashShift(r.Context(), in)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"shift_id": id, "status": "open"})
}

func (s *Server) handleCashMovement(w http.ResponseWriter, r *http.Request) {
	var in commands.CashMovementInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid cash movement payload"})
		return
	}
	id, err := s.processor.RecordCashMovement(r.Context(), in)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"movement_id": id})
}

func (s *Server) handleCloseShift(w http.ResponseWriter, r *http.Request) {
	var in commands.CloseShiftInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid close shift payload"})
		return
	}
	res, err := s.processor.CloseCashShift(r.Context(), in)
	if err != nil {
		if errors.Is(err, commands.ErrNotFound) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "open cash shift not found"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func (s *Server) handleVoidRefund(w http.ResponseWriter, r *http.Request) {
	var in commands.VoidRefundInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid void/refund payload"})
		return
	}
	auditID, err := s.processor.VoidOrRefundOrder(r.Context(), in)
	if err != nil {
		if errors.Is(err, commands.ErrNotFound) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "order not found"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"audit_id": auditID, "status": in.ActionType})
}

func (s *Server) handleOfflinePINPunch(w http.ResponseWriter, r *http.Request) {
	var in struct {
		PIN            string `json:"pin"`
		PunchType      string `json:"punch_type"`
		IdempotencyKey string `json:"idempotency_key"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid pin punch payload"})
		return
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if in.IdempotencyKey == "" {
		in.IdempotencyKey = "punch-" + now
	}
	payload, _ := json.Marshal(in)
	_, err := s.store.DB().ExecContext(r.Context(), `
		INSERT OR IGNORE INTO local_outbox (event_id, command_type, aggregate_type, aggregate_id, payload_json, idempotency_key, status, created_at)
		VALUES (?, 'hr.pin_punch', 'attendance', ?, ?, ?, 'pending', ?)
	`, in.IdempotencyKey, in.IdempotencyKey, string(payload), in.IdempotencyKey, now)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "queued_offline", "punch_type": in.PunchType})
}
