package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
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

	if os.Getenv("CLOUD_API_URL") == "" {
		_ = os.Setenv("CLOUD_API_URL", "https://ripedesk-cloud-api.onrender.com")
	}

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

	// Always overwrite branch_state from non-empty environment variables on startup
	cleanEnv := func(k string) string {
		return strings.Trim(strings.TrimSpace(os.Getenv(k)), `"'`)
	}
	cloudURL := cleanEnv("CLOUD_API_URL")
	tenantID := cleanEnv("TENANT_ID")
	branchID := cleanEnv("BRANCH_ID")
	deviceID := cleanEnv("BRANCH_DEVICE_ID")
	deviceSecret := cleanEnv("BRANCH_DEVICE_SECRET")
	authToken := cleanEnv("BRANCH_AUTH_TOKEN")

	if cloudURL != "" || tenantID != "" || branchID != "" || deviceID != "" || deviceSecret != "" {
		if _, err := store.DB().Exec(`
			UPDATE branch_state
			SET cloud_api_url = CASE WHEN ? != '' THEN ? ELSE cloud_api_url END,
			    tenant_id     = CASE WHEN ? != '' THEN ? ELSE tenant_id END,
			    branch_id     = CASE WHEN ? != '' THEN ? ELSE branch_id END,
			    device_id     = CASE WHEN ? != '' THEN ? ELSE device_id END,
			    device_secret = CASE WHEN ? != '' THEN ? ELSE device_secret END
			WHERE id = 1
		`, cloudURL, cloudURL, tenantID, tenantID, branchID, branchID, deviceID, deviceID, deviceSecret, deviceSecret); err != nil {
			logger.Warn("failed to hydrate branch_state from env", "error", err)
		}
	}
	logger.Info("branch_config_loaded",
		"tenant_id", tenantID,
		"branch_id", branchID,
		"cloud_api_url", cloudURL,
		"device_id", deviceID,
		"device_secret_set", deviceSecret != "",
		"auth_token_set", authToken != "",
	)

	hub := lan.NewHub()
	pub := &hubPublisher{hub: hub}
	printSpooler := spooler.NewSpooler(store)
	processor := commands.NewProcessor(store, pub)
	processor.SetSpooler(printSpooler)
	server := lan.NewServer(addr, store, hub, processor, printSpooler)
	syncEngine := branchsync.NewEngine(store, pub, authToken)

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
		if err := syncEngine.SendHeartbeat(ctx); err != nil {
			logger.Warn("initial_cloud_heartbeat_failed", "error", err)
		} else {
			logger.Info("initial_cloud_heartbeat_ok", "cloud_api_url", cloudURL, "branch_id", branchID)
		}

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
				if shipped, err := syncEngine.ShipNextBatch(ctx); err != nil {
					logger.Warn("outbox_batch_ship_failed", "error", err)
				} else if shipped > 0 {
					logger.Info("outbox_batch_shipped", "count", shipped)
				}
			case <-heartbeatTicker.C:
				if err := syncEngine.SendHeartbeat(ctx); err != nil {
					logger.Warn("cloud_heartbeat_failed", "error", err)
				}
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
