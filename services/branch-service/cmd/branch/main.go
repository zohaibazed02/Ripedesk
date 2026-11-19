package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/hospitality-os/branch-service/internal/commands"
	"github.com/hospitality-os/branch-service/internal/hardware/spooler"
	"github.com/hospitality-os/branch-service/internal/lan"
	"github.com/hospitality-os/branch-service/internal/sqlite"
	branchsync "github.com/hospitality-os/branch-service/internal/sync"
)

type hubPublisher struct {
	hub *lan.Hub
}

func (p *hubPublisher) PublishEvent(ev commands.LANEvent) {
	p.hub.Publish(lan.Event{
		SeqID:       ev.SeqID,
		EventType:   ev.EventType,
		AggregateID: ev.AggregateID,
		PayloadJSON: ev.PayloadJSON,
		CreatedAt:   ev.CreatedAt,
	})
}

func (p *hubPublisher) BroadcastSyncStatus(cloudReachable bool, pendingCount int) {
	p.hub.Publish(lan.Event{
		EventType:   "sync.status_changed",
		AggregateID: "branch",
		PayloadJSON: `{"cloud_reachable":` + boolToStr(cloudReachable) + `}`,
		CreatedAt:   time.Now().UTC().Format(time.RFC3339Nano),
	})
}

func boolToStr(v bool) string {
	if v {
		return "true"
	}
	return "false"
}

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	addr := os.Getenv("BRANCH_SERVICE_ADDR")
	if addr == "" {
		addr = ":8090"
	}
	dbPath := os.Getenv("BRANCH_SQLITE_PATH")
	if dbPath == "" {
		dbPath = "./branch_local.db"
	}

	store, err := sqlite.Open(dbPath)
	if err != nil {
		logger.Error("failed to open sqlite WAL store", "path", dbPath, "error", err)
		os.Exit(1)
	}
	defer store.Close()

	hub := lan.NewHub()
	pub := &hubPublisher{hub: hub}
	printSpooler := spooler.NewSpooler(store)
	processor := commands.NewProcessor(store, pub)
	processor.SetSpooler(printSpooler)
	server := lan.NewServer(addr, store, hub, processor, printSpooler)
	syncEngine := branchsync.NewEngine(store, pub, os.Getenv("BRANCH_AUTH_TOKEN"))

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Background durable print spooler worker
	go func() {
		printTicker := time.NewTicker(1 * time.Second)
		defer printTicker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-printTicker.C:
				if printed, err := printSpooler.ProcessPending(ctx); err == nil && printed > 0 {
					logger.Info("print_jobs_completed", "count", printed)
				}
			}
		}
	}()

	// Background outbox shipper, heartbeat & catalog refresh loop
	go func() {
		shipTicker := time.NewTicker(3 * time.Second)
		heartbeatTicker := time.NewTicker(15 * time.Second)
		catalogTicker := time.NewTicker(60 * time.Second)
		defer shipTicker.Stop()
		defer heartbeatTicker.Stop()
		defer catalogTicker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-shipTicker.C:
				if shipped, err := syncEngine.ShipNextBatch(ctx); err == nil && shipped > 0 {
					logger.Info("outbox_batch_shipped", "count", shipped)
				}
			case <-heartbeatTicker.C:
				_ = syncEngine.SendHeartbeat(ctx)
			case <-catalogTicker.C:
				_ = syncEngine.RefreshCatalog(ctx)
			}
		}
	}()

	go func() {
		logger.Info("branch-service LAN node starting", "addr", addr, "sqlite", store.Path())
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("branch-service server error", "error", err)
			stop()
		}
	}()

	<-ctx.Done()
	logger.Info("branch-service draining SSE connections and shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Error("branch-service forced shutdown", "error", err)
	}
}
