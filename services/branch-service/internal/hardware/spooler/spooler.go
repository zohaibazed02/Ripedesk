package spooler

import (
	"context"
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/hospitality-os/branch-service/internal/hardware/escpos"
	"github.com/hospitality-os/branch-service/internal/sqlite"
)

type Printer struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	Station       string `json:"station"`
	TransportType string `json:"transport_type"`
	Address       string `json:"address"`
	PaperWidthMM  int    `json:"paper_width_mm"`
	IsActive      bool   `json:"is_active"`
}

type PrintJob struct {
	ID             string `json:"id"`
	IdempotencyKey string `json:"idempotency_key"`
	PrinterID      string `json:"printer_id"`
	JobType        string `json:"job_type"`
	Status         string `json:"status"`
	Attempts       int    `json:"attempts"`
	MaxAttempts    int    `json:"max_attempts"`
	LastError      string `json:"last_error,omitempty"`
	CreatedAt      string `json:"created_at"`
	PrintedAt      string `json:"printed_at,omitempty"`
}

type Spooler struct {
	store      *sqlite.Store
	mu         sync.RWMutex
	transports map[string]Transport
}

func NewSpooler(store *sqlite.Store) *Spooler {
	return &Spooler{
		store:      store,
		transports: make(map[string]Transport),
	}
}

func (s *Spooler) RegisterTransport(printerID string, tr Transport) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.transports[printerID] = tr
}

func (s *Spooler) resolveTransport(p Printer) Transport {
	s.mu.RLock()
	tr, ok := s.transports[p.ID]
	s.mu.RUnlock()
	if ok {
		return tr
	}
	switch p.TransportType {
	case "network_tcp":
		return &NetworkTCPTransport{Address: p.Address}
	case "os_raw":
		return &OSRawSpoolerTransport{DevicePath: p.Address}
	default:
		mock := NewVirtualMockPrinterTransport()
		s.RegisterTransport(p.ID, mock)
		return mock
	}
}

func (s *Spooler) RegisterPrinter(ctx context.Context, p Printer) (*Printer, error) {
	if p.ID == "" {
		p.ID = uuid.NewString()
	}
	if p.PaperWidthMM != 58 {
		p.PaperWidthMM = 80
	}
	activeInt := 1
	if !p.IsActive {
		activeInt = 0
	}
	_, err := s.store.DB().ExecContext(ctx, `
		INSERT INTO print_printers (id, name, station, transport_type, address, paper_width_mm, is_active)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT (id) DO UPDATE SET
			name = EXCLUDED.name,
			station = EXCLUDED.station,
			transport_type = EXCLUDED.transport_type,
			address = EXCLUDED.address,
			paper_width_mm = EXCLUDED.paper_width_mm,
			is_active = EXCLUDED.is_active
	`, p.ID, p.Name, p.Station, p.TransportType, p.Address, p.PaperWidthMM, activeInt)
	if err != nil {
		return nil, err
	}
	return &p, nil
}

func (s *Spooler) ListPrinters(ctx context.Context) ([]Printer, error) {
	rows, err := s.store.DB().QueryContext(ctx, `
		SELECT id, name, station, transport_type, address, paper_width_mm, is_active
		FROM print_printers ORDER BY station ASC, name ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var printers []Printer
	for rows.Next() {
		var p Printer
		var activeInt int
		if err := rows.Scan(&p.ID, &p.Name, &p.Station, &p.TransportType, &p.Address, &p.PaperWidthMM, &activeInt); err == nil {
			p.IsActive = activeInt == 1
			printers = append(printers, p)
		}
	}
	return printers, nil
}

// EnqueueJob inserts a durable print job guarded by UNIQUE idempotency_key.
// If the idempotency_key already exists, it returns the existing job ID without duplicating.
func (s *Spooler) EnqueueJob(ctx context.Context, printerID, jobType, idempotencyKey string, escposBytes []byte) (string, error) {
	if idempotencyKey == "" {
		idempotencyKey = "pj-" + uuid.NewString()
	}
	var existingID string
	err := s.store.DB().QueryRowContext(ctx, `
		SELECT id FROM print_jobs WHERE idempotency_key = ?
	`, idempotencyKey).Scan(&existingID)
	if err == nil {
		return existingID, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return "", err
	}

	jobID := uuid.NewString()
	now := time.Now().UTC().Format(time.RFC3339Nano)
	encoded := base64.StdEncoding.EncodeToString(escposBytes)

	_, err = s.store.DB().ExecContext(ctx, `
		INSERT INTO print_jobs (id, idempotency_key, printer_id, job_type, escpos_base64, status, attempts, max_attempts, created_at)
		VALUES (?, ?, ?, ?, ?, 'queued', 0, 5, ?)
	`, jobID, idempotencyKey, printerID, jobType, encoded, now)
	if err != nil {
		return "", err
	}
	return jobID, nil
}

// EnqueueForStation looks up the active printer for a station and enqueues the job if configured.
func (s *Spooler) EnqueueForStation(ctx context.Context, station, jobType, idempotencyKey string, escposBytes []byte) (string, error) {
	var printerID string
	err := s.store.DB().QueryRowContext(ctx, `
		SELECT id FROM print_printers WHERE station = ? AND is_active = 1 LIMIT 1
	`, station).Scan(&printerID)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return s.EnqueueJob(ctx, printerID, jobType, idempotencyKey, escposBytes)
}

func (s *Spooler) KickCashDrawer(ctx context.Context, idempotencyKey string) (string, error) {
	return s.EnqueueForStation(ctx, "receipt_counter", "cash_drawer_kick", idempotencyKey, escpos.RenderCashDrawerPulse())
}

// ProcessPending drains queued / failed_retry jobs whose next_retry_at <= now.
func (s *Spooler) ProcessPending(ctx context.Context) (int, error) {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	rows, err := s.store.DB().QueryContext(ctx, `
		SELECT j.id, j.printer_id, j.escpos_base64, j.attempts, j.max_attempts,
		       p.name, p.station, p.transport_type, p.address, p.paper_width_mm
		FROM print_jobs j
		JOIN print_printers p ON p.id = j.printer_id
		WHERE j.status IN ('queued', 'failed_retry')
		  AND (j.next_retry_at IS NULL OR j.next_retry_at <= ?)
		ORDER BY j.created_at ASC
		LIMIT 25
	`, now)
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	type pendingJob struct {
		jobID        string
		escposBase64 string
		attempts     int
		maxAttempts  int
		printer      Printer
	}
	var jobs []pendingJob
	for rows.Next() {
		var pj pendingJob
		if err := rows.Scan(
			&pj.jobID, &pj.printer.ID, &pj.escposBase64, &pj.attempts, &pj.maxAttempts,
			&pj.printer.Name, &pj.printer.Station, &pj.printer.TransportType, &pj.printer.Address, &pj.printer.PaperWidthMM,
		); err == nil {
			jobs = append(jobs, pj)
		}
	}
	rows.Close()

	printedCount := 0
	for _, pj := range jobs {
		frame, err := base64.StdEncoding.DecodeString(pj.escposBase64)
		if err != nil {
			_, _ = s.store.DB().ExecContext(ctx, `
				UPDATE print_jobs SET status = 'dead_letter', last_error = ? WHERE id = ?
			`, "invalid base64 payload", pj.jobID)
			continue
		}

		_, _ = s.store.DB().ExecContext(ctx, `UPDATE print_jobs SET status = 'printing', attempts = attempts + 1 WHERE id = ?`, pj.jobID)
		tr := s.resolveTransport(pj.printer)

		if err := tr.Write(ctx, frame); err != nil {
			nextAttempt := pj.attempts + 1
			if nextAttempt >= pj.maxAttempts {
				_, _ = s.store.DB().ExecContext(ctx, `
					UPDATE print_jobs SET status = 'dead_letter', last_error = ? WHERE id = ?
				`, err.Error(), pj.jobID)
			} else {
				backoff := time.Duration(1<<minInt(nextAttempt, 5)) * time.Second
				nextRetry := time.Now().UTC().Add(backoff).Format(time.RFC3339Nano)
				_, _ = s.store.DB().ExecContext(ctx, `
					UPDATE print_jobs SET status = 'failed_retry', last_error = ?, next_retry_at = ? WHERE id = ?
				`, err.Error(), nextRetry, pj.jobID)
			}
			continue
		}

		printedAt := time.Now().UTC().Format(time.RFC3339Nano)
		_, _ = s.store.DB().ExecContext(ctx, `
			UPDATE print_jobs SET status = 'printed', last_error = '', printed_at = ? WHERE id = ?
		`, printedAt, pj.jobID)
		printedCount++
	}
	return printedCount, nil
}

func (s *Spooler) ListJobs(ctx context.Context, statusFilter string) ([]PrintJob, error) {
	query := `
		SELECT id, idempotency_key, printer_id, job_type, status, attempts, max_attempts, last_error, created_at, COALESCE(printed_at, '')
		FROM print_jobs
	`
	var args []any
	if statusFilter != "" {
		query += ` WHERE status = ?`
		args = append(args, statusFilter)
	}
	query += ` ORDER BY created_at DESC LIMIT 100`

	rows, err := s.store.DB().QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var jobs []PrintJob
	for rows.Next() {
		var j PrintJob
		if err := rows.Scan(&j.ID, &j.IdempotencyKey, &j.PrinterID, &j.JobType, &j.Status, &j.Attempts, &j.MaxAttempts, &j.LastError, &j.CreatedAt, &j.PrintedAt); err == nil {
			jobs = append(jobs, j)
		}
	}
	return jobs, nil
}

func (s *Spooler) ReprintJob(ctx context.Context, jobID string) error {
	res, err := s.store.DB().ExecContext(ctx, `
		UPDATE print_jobs
		SET status = 'queued', attempts = 0, next_retry_at = NULL, last_error = ''
		WHERE id = ?
	`, jobID)
	if err != nil {
		return err
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		return fmt.Errorf("print job %s not found", jobID)
	}
	return nil
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
