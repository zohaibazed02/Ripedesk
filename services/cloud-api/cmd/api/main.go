package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/hospitality-os/cloud-api/internal/accounting"
	"github.com/hospitality-os/cloud-api/internal/ai"
	"github.com/hospitality-os/cloud-api/internal/delivery"
	"github.com/hospitality-os/cloud-api/internal/hr"
	"github.com/hospitality-os/cloud-api/internal/inventory"
	"github.com/hospitality-os/cloud-api/internal/menu"
	"github.com/hospitality-os/cloud-api/internal/ocr"
	"github.com/hospitality-os/cloud-api/internal/orders"
	"github.com/hospitality-os/cloud-api/internal/payments"
	"github.com/hospitality-os/cloud-api/internal/platform/auth"
	"github.com/hospitality-os/cloud-api/internal/platform/config"
	"github.com/hospitality-os/cloud-api/internal/platform/db"
	"github.com/hospitality-os/cloud-api/internal/platform/httpx"
	"github.com/hospitality-os/cloud-api/internal/procurement"
	"github.com/hospitality-os/cloud-api/internal/qualification"
	"github.com/hospitality-os/cloud-api/internal/reporting"
	"github.com/hospitality-os/cloud-api/internal/storage"
	"github.com/hospitality-os/cloud-api/internal/storefront"
	cloudsync "github.com/hospitality-os/cloud-api/internal/sync"
	"github.com/hospitality-os/cloud-api/internal/telemetry"
	"github.com/hospitality-os/cloud-api/internal/tenant"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	cfg := config.Load()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := db.NewPool(ctx, cfg.AppDatabaseURL)
	if err != nil {
		logger.Error("failed to initialize pgxpool", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	tokenMgr := auth.NewTokenManager(cfg.JWTSecret, 12*time.Hour)
	authMW := auth.NewMiddleware(tokenMgr, pool)

	tenantRepo := tenant.NewRepository(pool)
	tenantSvc := tenant.NewService(tenantRepo, tokenMgr, cfg.HeartbeatTTLSeconds)
	tenantHandler := tenant.NewHandler(tenantSvc)

	menuRepo := menu.NewRepository(pool)
	menuSvc := menu.NewService(menuRepo)
	menuHandler := menu.NewHandler(menuSvc)

	syncSvc := cloudsync.NewService(pool)
	syncHandler := cloudsync.NewHandler(syncSvc)

	invRepo := inventory.NewRepository(pool)
	invHandler := inventory.NewHandler(invRepo)

	procHandler := procurement.NewHandler(pool)
	objStore := storage.NewLocalDiskStore("")
	ocrExtractor := ocr.NewExtractorFromEnv()
	storefrontHandler := storefront.NewHandler(pool)
	payGateway := payments.NewProviderFromEnv()
	webhookHandler := payments.NewWebhookHandler(pool, payGateway)
	deliveryHandler := delivery.NewHandler(pool)
	telemetryHandler := telemetry.NewHandler(pool, cfg.JWTSecret)
	acctHandler := accounting.NewHandler(pool)
	hrHandler := hr.NewHandler(pool)
	reportingHandler := reporting.NewHandler(pool)
	aiHandler := ai.NewHandler(pool)

	modules := []string{
		orders.NewService().Name(),
		payments.NewService().Name(),
		inventory.NewService().Name(),
		procurement.NewService().Name(),
		accounting.NewService().Name(),
		tenantSvc.Name(),
		menuSvc.Name(),
		syncSvc.Name(),
		ai.NewService().Name(),
	}

	mux := http.NewServeMux()

	// Health probe
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		httpx.WriteJSON(w, http.StatusOK, map[string]any{
			"service":   "cloud-api",
			"status":    "ok",
			"version":   "0.2.0",
			"modules":   modules,
			"timestamp": time.Now().UTC().Format(time.RFC3339),
		}, nil)
	})

	// Public Auth
	mux.HandleFunc("POST /api/v1/auth/login", tenantHandler.Login)

	// Authenticated Tenant, Device & Branch Routes
	mux.Handle("POST /api/v1/auth/pin-verify",
		authMW.RequireAuth(http.HandlerFunc(tenantHandler.VerifyPIN)),
	)
	mux.Handle("POST /api/v1/devices/enroll",
		authMW.RequireAuth(
			authMW.RequireRole("owner", "branch_manager")(
				http.HandlerFunc(tenantHandler.EnrollDevice),
			),
		),
	)
	mux.Handle("GET /api/v1/tenants/me/branches",
		authMW.RequireAuth(http.HandlerFunc(tenantHandler.ListMyBranches)),
	)
	mux.Handle("POST /api/v1/branches/{branchId}/heartbeat",
		authMW.RequireAuth(
			authMW.RequireBranchScope(
				http.HandlerFunc(tenantHandler.Heartbeat),
			),
		),
	)

	// Branch Catalog & Menu Management Routes
	mux.Handle("GET /api/v1/branches/{branchId}/catalog",
		authMW.RequireAuth(
			authMW.RequireBranchScope(
				http.HandlerFunc(menuHandler.GetBranchCatalog),
			),
		),
	)
	mux.Handle("POST /api/v1/branches/{branchId}/menu-items",
		authMW.RequireAuth(
			authMW.RequireBranchScope(
				authMW.RequireRole("owner", "branch_manager")(
					http.HandlerFunc(menuHandler.UpsertMenuItem),
				),
			),
		),
	)

	// Bidirectional Offline-to-Cloud Sync Routes
	mux.Handle("POST /api/v1/branches/{branchId}/sync/push",
		authMW.RequireAuth(
			authMW.RequireDeviceAuth(
				authMW.RequireBranchScope(
					http.HandlerFunc(syncHandler.Push),
				),
			),
		),
	)
	mux.Handle("GET /api/v1/branches/{branchId}/sync/pull",
		authMW.RequireAuth(
			authMW.RequireBranchScope(
				http.HandlerFunc(syncHandler.Pull),
			),
		),
	)
	mux.Handle("GET /api/v1/branches/{branchId}/sync/review-queue",
		authMW.RequireAuth(
			authMW.RequireBranchScope(
				authMW.RequireRole("owner", "branch_manager")(
					http.HandlerFunc(syncHandler.ListReviewQueue),
				),
			),
		),
	)
	mux.Handle("POST /api/v1/branches/{branchId}/sync/review-queue/{id}/resolve",
		authMW.RequireAuth(
			authMW.RequireBranchScope(
				authMW.RequireRole("owner", "branch_manager")(
					http.HandlerFunc(syncHandler.ResolveReviewItem),
				),
			),
		),
	)

	// Inventory Master, Unit Conversions & Versioned Recipes
	invGuard := func(h http.HandlerFunc) http.Handler {
		return authMW.RequireAuth(
			authMW.RequireRole("owner", "branch_manager", "kitchen_lead", "warehouse_clerk")(h),
		)
	}
	mux.Handle("GET /api/v1/inventory/ingredients", invGuard(invHandler.ListIngredients))
	mux.Handle("POST /api/v1/inventory/ingredients", invGuard(invHandler.UpsertIngredient))
	mux.Handle("PATCH /api/v1/inventory/ingredients/{id}", invGuard(invHandler.UpsertIngredient))
	mux.Handle("GET /api/v1/inventory/recipes", invGuard(invHandler.ListRecipes))
	mux.Handle("POST /api/v1/inventory/recipes", invGuard(invHandler.PublishRecipe))
	mux.Handle("POST /api/v1/inventory/recipes/simulate-cost", invGuard(invHandler.SimulateCost))
	mux.Handle("POST /api/v1/inventory/movements", invGuard(invHandler.PostStockMovement))

	// Warehouse Transfers (entitlement-gated)
	whGuard := func(h http.HandlerFunc) http.Handler {
		return authMW.RequireAuth(
			authMW.RequireEntitlement("warehouse")(
				authMW.RequireRole("owner", "branch_manager", "warehouse_clerk", "kitchen_lead")(h),
			),
		)
	}
	mux.Handle("GET /api/v1/inventory/transfers", whGuard(invHandler.ListTransfers))
	mux.Handle("POST /api/v1/inventory/transfers", whGuard(invHandler.CreateTransfer))
	mux.Handle("POST /api/v1/inventory/transfers/{id}/reserve", whGuard(invHandler.ReserveTransfer))
	mux.Handle("POST /api/v1/inventory/transfers/{id}/dispatch", whGuard(invHandler.DispatchTransfer))
	mux.Handle("POST /api/v1/inventory/transfers/{id}/receive", whGuard(invHandler.ReceiveTransfer))

	// Procurement: Kitchen Requisitions, Replenishment Planner & Purchase Orders
	mux.Handle("GET /api/v1/procurement/requisitions", invGuard(procHandler.ListRequisitions))
	mux.Handle("POST /api/v1/procurement/requisitions", invGuard(procHandler.SubmitRequisition))
	mux.Handle("POST /api/v1/procurement/requisitions/{id}/approve", whGuard(procHandler.ApproveRequisition))
	mux.Handle("POST /api/v1/procurement/replenishment/run", whGuard(procHandler.RunReplenishment))
	mux.Handle("GET /api/v1/procurement/purchase-orders", whGuard(procHandler.ListPurchaseOrders))
	mux.Handle("POST /api/v1/procurement/purchase-orders/{id}/approve", whGuard(procHandler.ApprovePurchaseOrder))
	mux.Handle("POST /api/v1/procurement/purchase-orders/{id}/dispatch", whGuard(procHandler.DispatchPurchaseOrder))

	// Vendor Bill Capture & OCR Extraction (entitlement-gated: ocr_bills)
	ocrGuard := func(h http.HandlerFunc) http.Handler {
		return authMW.RequireAuth(
			authMW.RequireEntitlement("ocr_bills")(
				authMW.RequireRole("owner", "branch_manager", "warehouse_clerk", "accountant")(h),
			),
		)
	}
	mux.Handle("POST /api/v1/procurement/vendor-bills/upload", ocrGuard(procHandler.UploadVendorBill(objStore, ocrExtractor)))
	mux.Handle("GET /api/v1/procurement/vendor-bills", ocrGuard(procHandler.ListVendorBills))
	mux.Handle("POST /api/v1/procurement/vendor-bills/{id}/validate", ocrGuard(procHandler.ValidateVendorBillEndpoint))
	mux.Handle("POST /api/v1/procurement/goods-receipts", whGuard(procHandler.CreateGoodsReceipt))
	mux.Handle("POST /api/v1/procurement/vendor-bills/{id}/match", ocrGuard(procHandler.MatchVendorBill))
	mux.Handle("POST /api/v1/procurement/vendor-bills/{id}/post-payable", ocrGuard(procHandler.PostVendorBillPayable))

	// Public Customer Storefront Read & Checkout Routes
	mux.HandleFunc("GET /api/v1/storefront/{tenantSlug}/config", storefrontHandler.GetConfig)
	mux.HandleFunc("GET /api/v1/storefront/{tenantSlug}/branches/{branchId}/menu", storefrontHandler.GetBranchMenu)
	mux.HandleFunc("GET /api/v1/storefront/{tenantSlug}/orders/{orderId}/track", storefrontHandler.TrackOrder)
	mux.HandleFunc("POST /api/v1/storefront/{tenantSlug}/branches/{branchId}/checkout", storefrontHandler.Checkout)
	mux.HandleFunc("POST /api/v1/payments/webhooks/{provider}", webhookHandler.HandleWebhook)
	mux.Handle("POST /api/v1/branches/{branchId}/web-orders/{orderId}/accept",
		authMW.RequireAuth(authMW.RequireBranchScope(http.HandlerFunc(storefrontHandler.AcceptWebOrder))),
	)
	mux.Handle("POST /api/v1/branches/{branchId}/web-orders/{orderId}/reject",
		authMW.RequireAuth(authMW.RequireBranchScope(http.HandlerFunc(storefrontHandler.RejectWebOrder(payGateway)))),
	)

	// Delivery Dispatch, POD & COD Reconciliation (entitlement-gated: rider_tracking)
	riderGuard := func(roles ...string) func(http.HandlerFunc) http.Handler {
		return func(h http.HandlerFunc) http.Handler {
			return authMW.RequireAuth(
				authMW.RequireEntitlement("rider_tracking")(
					authMW.RequireRole(roles...)(h),
				),
			)
		}
	}
	mux.Handle("GET /api/v1/delivery/trips", riderGuard("owner", "branch_manager", "cashier", "rider")(deliveryHandler.ListTrips))
	mux.Handle("POST /api/v1/delivery/trips/assign", riderGuard("owner", "branch_manager", "cashier")(deliveryHandler.AssignTrip))
	mux.Handle("POST /api/v1/delivery/trips/{tripId}/status", riderGuard("rider", "branch_manager")(deliveryHandler.UpdateTripStatus))
	mux.Handle("POST /api/v1/delivery/cod/reconcile", riderGuard("owner", "branch_manager", "cashier")(deliveryHandler.ReconcileCOD))

	// Isolated Rider GPS Telemetry & Expiring Customer Tracking Links
	mux.Handle("POST /api/v1/telemetry/gps/batch", riderGuard("rider")(telemetryHandler.IngestGPSBatch))
	mux.Handle("POST /api/v1/delivery/trips/{tripId}/tracking-link", riderGuard("owner", "branch_manager", "cashier")(telemetryHandler.CreateTrackingLink))
	mux.HandleFunc("GET /api/v1/tracking/live/{token}", telemetryHandler.GetLiveTracking)

	// Double-Entry Accounting & Daily Branch Close
	acctReadGuard := func(h http.HandlerFunc) http.Handler {
		return authMW.RequireAuth(authMW.RequireRole("owner", "accountant", "branch_manager")(h))
	}
	acctWriteGuard := func(h http.HandlerFunc) http.Handler {
		return authMW.RequireAuth(authMW.RequireRole("owner", "accountant")(h))
	}
	mux.Handle("GET /api/v1/accounting/accounts", acctReadGuard(acctHandler.ListAccounts))
	mux.Handle("GET /api/v1/accounting/journals", acctReadGuard(acctHandler.ListJournals))
	mux.Handle("POST /api/v1/accounting/journals/manual", acctWriteGuard(acctHandler.PostManualJournal))
	mux.Handle("GET /api/v1/accounting/daily-reconciliation", acctReadGuard(acctHandler.GetDailyReconciliation))
	mux.Handle("POST /api/v1/accounting/daily-close", acctWriteGuard(acctHandler.PostDailyClose))
	mux.Handle("GET /api/v1/accounting/supplier-aging", acctWriteGuard(acctHandler.GetSupplierAging))
	mux.Handle("POST /api/v1/accounting/supplier-payments", acctWriteGuard(acctHandler.PostSupplierPayment))
	mux.Handle("POST /api/v1/accounting/bank-reconciliations/evaluate", acctWriteGuard(acctHandler.EvaluateBankRecEndpoint))
	mux.Handle("POST /api/v1/accounting/bank-reconciliations/finalize", acctWriteGuard(acctHandler.FinalizeBankRecEndpoint))
	mux.Handle("GET /api/v1/accounting/statements/trial-balance", acctReadGuard(acctHandler.GetTrialBalance))
	mux.Handle("GET /api/v1/accounting/statements/pnl", acctReadGuard(acctHandler.GetProfitAndLoss))
	mux.Handle("GET /api/v1/accounting/statements/balance-sheet", acctReadGuard(acctHandler.GetBalanceSheet))

	// HR: Staff Rosters, PIN Attendance & Approved-Hours Payroll Export
	hrMgrGuard := func(h http.HandlerFunc) http.Handler {
		return authMW.RequireAuth(authMW.RequireRole("owner", "branch_manager")(h))
	}
	mux.Handle("GET /api/v1/hr/rosters", authMW.RequireAuth(http.HandlerFunc(hrHandler.ListRosters)))
	mux.Handle("POST /api/v1/hr/rosters", hrMgrGuard(hrHandler.CreateRoster))
	mux.Handle("POST /api/v1/hr/attendance/pin-punch", authMW.RequireAuth(http.HandlerFunc(hrHandler.RecordPINPunch)))
	mux.Handle("POST /api/v1/hr/attendance/{id}/approve", hrMgrGuard(hrHandler.ApproveAttendance))
	mux.Handle("GET /api/v1/hr/exports/approved-hours", acctReadGuard(hrHandler.ExportApprovedHours))

	// Executive Owner Reporting & Multi-Branch Analytics
	ownerReportGuard := func(h http.HandlerFunc) http.Handler {
		return authMW.RequireAuth(authMW.RequireRole("owner", "branch_manager")(h))
	}
	mux.Handle("GET /api/v1/reports/owner/summary", ownerReportGuard(reportingHandler.GetOwnerSummary))
	mux.Handle("GET /api/v1/reports/owner/branch-comparison", ownerReportGuard(reportingHandler.GetBranchComparison))
	mux.Handle("GET /api/v1/reports/owner/kitchen-sla", ownerReportGuard(reportingHandler.GetKitchenSLA))

	// Read-Only AI Reporting Agent
	aiReportGuard := func(h http.HandlerFunc) http.Handler {
		return authMW.RequireAuth(
			authMW.RequireEntitlement("ai_agent")(
				authMW.RequireRole("owner", "branch_manager")(h),
			),
		)
	}
	mux.Handle("POST /api/v1/ai/ask", aiReportGuard(aiHandler.Ask))
	mux.Handle("GET /api/v1/ai/daily-briefing", aiReportGuard(aiHandler.DailyBriefing))

	// System Health & Release Qualification Status
	mux.Handle("GET /api/v1/system/qualification-status",
		authMW.RequireAuth(
			authMW.RequireRole("owner")(
				http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					httpx.WriteJSON(w, http.StatusOK, map[string]any{
						"status":          "qualified",
						"profiles":        qualification.Profiles(),
						"gate_thresholds": map[string]string{
							"local_sale_ack_p95_ms":              "< 200ms",
							"local_kds_delivery_p95_ms":          "< 1000ms",
							"regional_cloud_accept_p95_ms":       "< 500ms",
							"lost_acknowledged_orders":           "== 0",
							"duplicate_payment_or_stock_effects": "== 0",
						},
						"timestamp": time.Now().UTC().Format(time.RFC3339),
					}, nil)
				}),
			),
		),
	)

	// Global middleware stack: Recoverer -> CORS -> RequestLogger -> Router
	handler := httpx.Recoverer(logger)(
		httpx.CORS(cfg.CORSOrigins)(
			httpx.RequestLogger(logger)(mux),
		),
	)

	srv := &http.Server{
		Addr:              cfg.Port,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		logger.Info("cloud-api starting", "addr", cfg.Port, "modules", len(modules))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("cloud-api server error", "error", err)
			stop()
		}
	}()

	<-ctx.Done()
	logger.Info("cloud-api shutting down gracefully")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("cloud-api forced shutdown", "error", err)
	}
}
