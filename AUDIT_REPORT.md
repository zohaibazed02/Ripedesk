# Hospitality OS — Codebase Audit Report

Generated: 2026-11-20 (Read-Only Audit)

---

## Section 1 — Pass 1 of 3: Architecture, Multi-Tenancy, Offline Sync & Gate 1 (Pages 1–3)

### 1. Repository & Stack Inventory — **PASS**

| Layer | Component | Path | Status |
| --- | --- | --- | --- |
| **Go Cloud API** | Modular Monolith (`api`, `worker`, `migrate`, `seed`, `loadtest`, `gen-branch-token`) | `services/cloud-api/` | **PASS** |
| **Go Branch LAN** | Offline-first SQLite WAL LAN Node (`branch`) | `services/branch-service/` | **PASS** |
| **Flutter Shared** | Shared Dart Domain Models, `Decimal` Pricing, Drift SQLite, LAN Client, Hardware Adapters | `apps/pos-flutter/packages/pos_domain/` | **PASS** |
| **Flutter Apps** | Windows POS (`windows_pos`), Android Staff (`android_staff`), Rider App (`rider_app`) | `apps/pos-flutter/apps/` | **PASS** |
| **Next.js Apps** | Admin ERP (`web-admin`), Storefront (`web-storefront`), KDS (`web-kds`), Owner PWA (`web-owner`) | `apps/web-*` | **PASS** |
| **Shared Packages** | Design System (`@hospitality/ui`), OpenAPI 3.1 Spec (`packages/openapi/openapi.yaml`) | `packages/` | **PASS** |
| **SQL Migrations** | 4 numbered up/down PostgreSQL migration pairs (`000001`–`000004`) | `services/cloud-api/migrations/` | **PASS** |

- **Go Modular Monolith Boundaries — PASS**: Distinct domain packages exist under `services/cloud-api/internal/`: `orders`, `payments`, `inventory`, `procurement`, `accounting`, `tenant`, `menu`, `sync`, `storefront`, `delivery`, `telemetry`, `hr`, `reporting`, `ai`, `ocr`, `storage`, `qualification`, plus `platform/{auth,config,db,httpx}`.
- **Flutter Shared Domain + Drift SQLite — PASS**: `apps/pos-flutter/packages/pos_domain/lib/db/pos_database.dart` (Drift tables), `lib/pricing/order_calculator.dart` (`decimal` package), `lib/sync/branch_lan_client.dart`, and `lib/hardware/{pos_printer_adapter,payment_terminal_adapter,barcode_scanner_listener}.dart`.
- **Next.js + OpenAPI Contracts — PASS**: `packages/openapi/openapi.yaml` documents API endpoints across Tenant, Menu, Sync, Inventory, Procurement, Storefront, Payments, Delivery, Telemetry, Accounting, HR, Reporting, and AI.

---

### 2. Database & Multi-Tenancy (PostgreSQL + SQLite) — **PASS**

| Check | Evidence (File & Line References) | Verdict |
| --- | --- | --- |
| **Tenant-Scoped Keys & Constraints** | `services/cloud-api/migrations/000001_tenant_and_rbac.up.sql` (L1–153), `000002_pos_and_orders.up.sql`, `000003_inventory_and_procurement.up.sql`, `000004_delivery_accounting_outbox.up.sql`: composite `(tenant_id, id)` unique keys & foreign keys across tenant tables. | **PASS** |
| **Fixed-Precision Money & Quantities (Zero Float)** | PostgreSQL uses `NUMERIC(14,4)` / `NUMERIC(12,4)` for money and quantities; Go uses `shopspring/decimal` across `inventory`, `procurement`, `accounting`, `reporting`, `ai`, `hr`, `qualification`; Dart uses `decimal` in `OrderCalculator`. | **PASS** |
| **PostgreSQL RLS Policies & Roles** | `services/cloud-api/migrations/000004_delivery_accounting_outbox.up.sql` enables `ROW LEVEL SECURITY` with `current_setting('app.current_tenant_id', true)::uuid` policies and non-superuser `pos_app_user`; enforced per-tx via `pool.WithTenantTx` (`services/cloud-api/internal/platform/db/pool.go`). Verified by `internal/tenant/rls_isolation_test.go`. | **PASS** |
| **Local SQLite WAL & Outbox Schema** | `services/branch-service/internal/sqlite/store.go` uses pure-Go `modernc.org/sqlite` in WAL mode with `branch_state`, `local_orders`, `local_kitchen_tickets`, `local_tables`, `lan_events`, `local_outbox`, and `print_jobs`. Flutter Drift schema in `apps/pos-flutter/packages/pos_domain/lib/db/pos_database.dart`. | **PASS** |

---

### 3. Offline Operations & Sync Guarantees — **PASS**

| Check | Evidence (File & Line References) | Verdict |
| --- | --- | --- |
| **Single-Tx Business Write + Outbox Insert** | `services/branch-service/internal/commands/processor.go` wraps order/ticket/table/LAN-event/outbox writes inside a single SQLite `BEGIN IMMEDIATE` transaction before acknowledging. Verified by `internal/commands/atomic_outbox_test.go`. | **PASS** |
| **Stable IDs & Idempotency Keys** | Client-generated UUIDs + `idempotency_key` deduplication in both `services/branch-service/internal/commands/processor.go` and `services/cloud-api/internal/sync/service.go` (`sync_idempotency_test.go`). | **PASS** |
| **Server-Side Device / Tenant / Branch Scope** | `services/cloud-api/internal/platform/auth/middleware.go` (L47–95): `RequireAuth` (Bearer JWT), `RequireDeviceAuth` (`X-Device-ID` + `X-Device-Signature` vs `devices.public_key_hash`), and `RequireBranchScope`. | **PASS** |
| **Batch Retry w/ Capped Exponential Backoff** | `services/branch-service/internal/sync/engine.go` (L116–235): `ComputeBackoff` (capped 60s + jitter), FIFO batching (`DefaultBatchSize = 50`), verified by `engine_replay_test.go`. | **PASS** |
| **Order Version Conflict Checks** | `services/branch-service/internal/commands/processor.go` rejects stale `expected_version` with `ErrVersionConflict` (`409 Conflict`); verified by `atomic_outbox_test.go` and `chaos/outage_drill_test.go` (Drill 4). | **PASS** |
| **Rejected-Command Review Queue** | `services/cloud-api/internal/sync/handler.go` (L64–104): `GET .../sync/review-queue` and `POST .../sync/review-queue/{id}/resolve`. | **PASS** |
| **Sync Health & Heartbeat Web-Order Pause** | `services/cloud-api/internal/tenant/repository.go` (L225–254) tracks `heartbeat_expires_at`, `last_sync_at`, `pending_sync_count`, `oldest_pending_tx_at`; `services/cloud-api/internal/storefront/checkout.go` (`checkout_handoff_test.go`) blocks immediate web checkout when branch heartbeat is expired. | **PASS** |

---

### 4. Pilot POS & KDS Features (Gate 1 Vertical Slice) — **PASS (with 1 note)**

| Feature | Evidence (File & Line References) | Verdict |
| --- | --- | --- |
| **Quick-Sale Default (Coffee Shop)** | `apps/pos-flutter/apps/windows_pos/lib/screens/quick_sale_screen.dart` & `lib/main.dart` (L53–58). | **PASS** |
| **Dine-In Table Map, Rounds & Split Checks** | `windows_pos/lib/screens/table_map_screen.dart`, `widgets/split_check_modal.dart`, `widgets/table_order_panel.dart`; `android_staff/lib/screens/{staff_tables_screen,tableside_order_screen}.dart`. | **PASS** |
| **Modifiers, Cash Shifts, Voids/Refunds, Receipts** | `windows_pos/lib/widgets/{modifier_drawer,cash_shift_modal,void_refund_modal,manager_pin_modal,tender_modal}.dart`; `services/branch-service/internal/commands/shift_and_audit.go`; `internal/hardware/escpos/{encoder,templates}.go`. | **PASS** |
| **Hardware Adapter Abstractions** | `pos_domain/lib/hardware/{pos_printer_adapter,payment_terminal_adapter,barcode_scanner_listener}.dart`; `services/branch-service/internal/hardware/spooler/spooler.go`. | **PASS** |
| **Web KDS Live LAN SSE + Polling Recovery** | `apps/web-kds/src/lib/kds-client.ts` (L30–32, `DEFAULT_LAN_URL`) & `apps/web-kds/src/app/page.tsx` (L34–93): connects to `/lan/stream` SSE + 3s `/lan/kds/tickets` polling, refreshes on `order.upserted`. **Note**: `windows_pos` `PosController` polls `/healthz` for online status; verify end-to-end `POST /lan/commands/orders` payload from `PosController` in live LAN testing. | **PASS** |

---

### 5. Build & Test Verification (Pass 1)

| Target | Command | Result |
| --- | --- | --- |
| **`services/cloud-api`** | `go test ./...` | **PASS** — all 13 test packages `ok` (`accounting`, `ai`, `delivery`, `hr`, `inventory`, `ocr`, `procurement`, `qualification`, `reporting`, `storefront`, `sync`, `telemetry`, `tenant`) |
| **`services/branch-service`** | `go test ./...` | **PASS** — all 4 test packages `ok` (`chaos`, `commands`, `hardware/spooler`, `sync`) |
| **`apps/web-kds`** | `npm.cmd run typecheck:kds && npm.cmd run build:kds` | **PASS** — `tsc --noEmit` & `next build` clean (4/4 static pages) |
| **Flutter (`windows_pos`, `android_staff`)** | `flutter analyze` / `flutter test` | **PARTIAL (Env)** — `flutter` SDK is not installed on this local Windows host (`Get-Command flutter` returned empty); Flutter CI runs on GitHub Actions (`.github/workflows/release-flutter.yml`). |

---

## Section 2 — Pass 2 of 3: Inventory, Warehouse, Procurement, OCR & 3-Way Match (Pages 4–6 & Gate 2)

### 1. Stock Ledger & Versioned Recipes — **PASS (with 1 partial)**

| Check | Evidence (File & Line References) | Verdict |
| --- | --- | --- |
| **Core Tables & Line-Level Links** | `services/cloud-api/migrations/000003_inventory_and_procurement.up.sql` (L1–396): `ingredients`, `supplier_items`, `unit_conversions`, `recipe_versions`, `recipe_lines`, `stock_movements`, `kitchen_requisitions`, `stock_transfers`, `purchase_orders`, `goods_receipts`, `vendor_invoices` (linked at line level with `tenant_id` composite FKs). | **PASS** |
| **Stock Movement Ledger (Location, Source, Base Unit)** | `services/cloud-api/internal/inventory/ledger.go` (L1–144): `PostStockMovementTx` records `location_id`, `movement_type` (`receipt`, `consumption`, `waste`, `adjustment`, `transfer_out`, `transfer_in`), `source_type`/`source_id`, `quantity_base`, and weighted average cost (`shopspring/decimal`). | **PASS** |
| **Versioned Recipe + Modifier Deduction & Production Batches** | `services/cloud-api/internal/inventory/recipes.go` (L1–131) & `production.go` (L1–82): `ExplodeRecipeWithModifiers` (tested on *Large Oat Latte + Extra Shot* in `recipe_and_conversion_test.go`), production batch yield rollup (`ledger_and_variance_test.go`). **Note**: Triggering recipe deduction automatically from KDS prep confirmation in `sync/service.go` is wired at domain level (`ExplodeRecipeWithModifiers`), not yet an end-to-end DB integration test against live Postgres. | **PARTIAL** |

---

### 2. Warehouse & Kitchen Requisition Workflow — **PASS**

| Check | Evidence (File & Line References) | Verdict |
| --- | --- | --- |
| **Full Transfer Lifecycle** | `services/cloud-api/internal/inventory/transfers.go` (L1–431): enforces `requested → approved → reserved → dispatched → in_transit → received` (and `partially_received`), deducting warehouse stock at dispatch and posting branch receipt via `PostStockMovementTx`, handling partial receipts, shortages, and damaged items. Verified by `transfer_workflow_test.go`. | **PASS** |
| **Kitchen Requisition Workflow & Statuses** | `services/cloud-api/internal/procurement/requisitions.go` (L1–217): supports `draft`, `submitted`, `approved`, `partially_fulfilled`, `fulfilled`, `rejected`, client-generated request IDs (`idempotency_key`), and split-sourcing (warehouse reservation + PO draft). Admin UI at `apps/web-admin/src/app/procurement/requisitions/page.tsx`. | **PASS** |

---

### 3. Automatic PO Drafts & Replay Idempotency — **PARTIAL**

| Check | Evidence (File & Line References) | Verdict |
| --- | --- | --- |
| **Replenishment Math (Stock, Reservations, Inbound, Lead Time, MOQ, Pack Size)** | `services/cloud-api/internal/procurement/replenishment.go` (L39–76): `EvaluateReplenishmentNeed` & `GroupReplenishmentDecisions` compute exact `Decimal` demand with lead time, safety stock, branch on-hand, open inbound, reservations, pack size, and MOQ. Verified by `TestReplenishmentPlannerAndGrouping`. | **PASS** |
| **12L Oat Milk / 8L Warehouse Split-Sourcing & Replay Idempotency** | `services/cloud-api/internal/procurement/requisitions.go` & `procurement_test.go` (`TestPilotSplitSourcingWithPackSizeAndMOQ`): verifies exact 8L warehouse reservation + 4L PO draft (pack-size/MOQ adjusted) in unit tests; DB-level `idempotency_key` unique constraint exists in `000003_inventory_and_procurement.up.sql`, though `RunReplenishment` HTTP handler (`replenishment.go` L78–88) currently returns computed groups without persisting PO drafts directly. | **PARTIAL** |
| **Outbound Supplier Delivery Tracking & Duplicate-Send Prevention** | `services/cloud-api/internal/procurement/purchase_orders.go` (L1–163) & `procurement_test.go` (`TestBuyerGuardrailsAndDuplicateDispatchProtection`): enforces approval guardrails and blocks duplicate dispatch of already-sent POs. | **PASS** |

---

### 4. Vendor Bill OCR & 3-Way Match — **PARTIAL (Stub OCR Adapter)**

| Check | Evidence (File & Line References) | Verdict |
| --- | --- | --- |
| **Upload, Blur Check, Duplicate Fingerprint & Normalized Invoice #** | `services/cloud-api/internal/ocr/quality.go` (Laplacian variance blur check, `TestBlurDetectionSharpVsBlurry`), `extractor.go` (L60–70: `NormalizeInvoiceNumber`, `ComputeDocumentSHA256`, `TestInvoiceNormalizationAndSHA256Fingerprint`), `storage/store.go` (local/S3-compatible object store). | **PASS** |
| **OCR Extraction Engine** | `services/cloud-api/internal/ocr/extractor.go`: `Extractor` interface is wired with `PilotBenchmarkExtractor` (deterministic benchmark fixture for Urdu/English bills) rather than a live external Vision/OCR API call. | **PARTIAL (Stub)** |
| **Supplier SKU Alias Mapping & Validation Workbench** | `services/cloud-api/internal/procurement/bill_validation.go` (L1–207) & `bill_validation_test.go` (`TestVendorBillValidationAndUrduAliasMapping`), UI at `apps/web-admin/src/app/procurement/bills/validate/page.tsx`. | **PASS** |
| **3-Way Match, Direct Purchase (No-PO) & Balanced Payable Journal** | `services/cloud-api/internal/procurement/three_way_match.go` (L1–259), `goods_receipt.go` (L1–106), `three_way_match_test.go` (`TestCleanThreeWayMatchPartialShipmentAndBalancedJournal`, `TestOverBilledQuantityHoldAndDirectPurchaseGuard`). Enforces manager auth + confirmed GRN for direct purchases and balanced `Decimal` journals. | **PASS** |
| **Untrusted OCR Text Sanitization Safeguard** | `services/cloud-api/internal/ocr/extractor.go` (L72–84): `SanitizeExtractedText` strips HTML/script tags and control characters; AI agent (`internal/ai/agent.go`) never ingests raw OCR text. Verified by `TestMathValidationUrduExtractionAndSanitization`. | **PASS** |

---

### 5. Unit & Integration Test Coverage (Gate 2)

All 15 Gate 2 Go unit tests executed via `go test -v ./internal/inventory/... ./internal/procurement/... ./internal/ocr/...` — **15/15 PASS**:
- **Covered by automated Go unit tests**: Weighted avg cost & production batch rollup, theoretical vs actual variance, multi-step pack conversion, *Large Oat Latte + Extra Shot* recipe explosion & COGS, transfer state machine, partial receipt/damage/shortage PO draft, vendor bill validation & Urdu alias mapping, pilot split-sourcing (pack size + MOQ), replenishment planner & grouping, buyer guardrails & duplicate PO dispatch protection, clean 3-way match + partial shipment + balanced journal, over-billed hold & direct purchase guard, blur detection, invoice normalization & SHA-256 fingerprint, OCR math validation & sanitization.
- **Lacking automated live-DB integration tests**: End-to-end PostgreSQL transaction replay of `RunReplenishment` persisting PO drafts, live external OCR provider HTTP adapter, and Playwright UI E2E tests for `apps/web-admin/src/app/procurement/*`.

---

## Section 3 — Pass 3 of 3: Online Ordering, Rider/Dispatch, Owner/HR/Accounting, AI Agent & Gate 5 (Pages 7–8 & Gates 3–5)

### 1. Online Ordering & Branded Templates (Gate 3) — **PARTIAL (Stub UI & Payment Gateway)**

| Check | Evidence (File & Line References) | Verdict |
| --- | --- | --- |
| **3 Storefront Themes on 1 Cart/Checkout Engine** | `apps/web-storefront/src/app/page.tsx`: Coffee Pickup, QSR Delivery/Combos, and Dine-In QR share one cart, modifier, and checkout flow; live tracking route at `apps/web-storefront/src/app/track/[orderId]/page.tsx`. | **PASS** |
| **Heartbeat Guard, Signed Webhooks & Interrupted Handoff Reversal** | `services/cloud-api/internal/storefront/{checkout.go,handoff.go,service.go}` & `internal/payments/{gateway.go,webhook_handler.go}`: blocks immediate web orders when branch heartbeat is expired, verifies HMAC-SHA256 webhook signatures, holds pending handoff until branch acceptance or reversal. Verified by `TestHeartbeatGuardWebhookSignatureAndInterruptedHandoffReversal`. | **PASS** |
| **Live Payment Provider Integration** | `services/cloud-api/internal/payments/gateway.go`: `OnlinePaymentProvider` interface is wired with `SimulatedGatewayProvider` rather than a live PSP (Stripe/PayFast/JazzCash). | **PARTIAL (Stub)** |
| **Storefront UI Live API Wiring** | `apps/web-storefront/src/app/page.tsx` renders static pilot menu fixtures instead of live `fetch()` calls to `GET /api/v1/storefront/{tenantSlug}/config` & `/menu`. | **PARTIAL (Stub)** |

---

### 2. Rider & Dispatch (Gate 3) — **PASS**

| Check | Evidence (File & Line References) | Verdict |
| --- | --- | --- |
| **Separate Rider App & Narrow Permissions** | `apps/pos-flutter/apps/rider_app/` (isolated Flutter app with `rider_controller.dart`, `active_trip_screen.dart`, `gps_telemetry_buffer.dart`); backend guards `rider` role in `cmd/api/main.go`. | **PASS** |
| **Dispatch, POD, COD Reconciliation & Weak-Connectivity Buffering** | `services/cloud-api/internal/delivery/trip.go` & `delivery_test.go` (`TestTripTransitionsPODEnforcementAndCODReconciliation`); Admin console at `apps/web-admin/src/app/delivery/dispatch/page.tsx`; Flutter FIFO buffer in `rider_app/lib/controllers/rider_controller.dart`. | **PASS** |
| **Separate GPS Telemetry Pipeline & Expiring Customer Links** | `services/cloud-api/internal/telemetry/{gps_service.go,tracking_token.go,telemetry_test.go}`: routed separately (`/api/v1/telemetry/gps/batch`), active-trip privacy gated, HMAC expiring tracking tokens verified by `TestPrivacyGuardOutOfOrderAndExpiringTrackingTokens`. | **PASS** |

---

### 3. Owner Reports, HR & Accounting (Gate 4) — **PARTIAL (Stub UI Wiring)**

| Check | Evidence (File & Line References) | Verdict |
| --- | --- | --- |
| **Owner Reporting Engine (Backend)** | `services/cloud-api/internal/reporting/{owner_metrics.go,branch_comparison.go,reporting_test.go}`: exact `shopspring/decimal` sales, collections, refunds, AOV, food cost %, waste %, variance %, kitchen SLA, POS-to-GL reconciliation, and >5m/`pending_sync_count` staleness flags. | **PASS** |
| **Owner PWA Live API Wiring** | `apps/web-owner/src/app/page.tsx` (L35–103) uses static `BASE_BRANCHES` demo metrics rather than live `fetch()` calls to `GET /api/v1/reports/owner/summary` & `/branch-comparison`. | **PARTIAL (Stub)** |
| **HR Rosters, PIN Clock-In, Exceptions & Approved-Hour Export** | `services/cloud-api/internal/hr/{rosters.go,attendance.go,payroll_export.go,hr_test.go}` & `services/branch-service/internal/commands/shift_and_audit.go`; Admin UI at `apps/web-admin/src/app/hr/attendance/page.tsx`. | **PASS** |
| **Double-Entry Accounting, Daily Close, Aging & Statements** | `services/cloud-api/internal/accounting/{coa.go,journal.go,daily_close.go,aging.go,bank_rec.go,statements.go}`: strictly balanced `Decimal` journals, daily reconciliation guards, supplier aging buckets, bank rec, Trial Balance / P&L / Balance Sheet. All 5 tests in `internal/accounting` PASS. | **PASS** |

---

### 4. Read-Only AI Reporting Agent (Gate 4) — **PASS (Backend) / PARTIAL (Stub UI)**

| Check | Evidence (File & Line References) | Verdict |
| --- | --- | --- |
| **Read-Only Tool Allowlist & Tenant/Branch RBAC** | `services/cloud-api/internal/ai/tools.go` (L25–155): strict 5-tool read-only allowlist (`ErrUnapprovedTool`), `ValidateScope` (`ErrForbiddenBranchScope` for cross-branch `branch_manager`), zero SQL writes. Verified by `ai_agent_test.go`. | **PASS** |
| **LLM Explainer & Grounded Response Contract** | `services/cloud-api/internal/ai/agent.go` (L68–180): `ManagedLLMHTTPAdapter` calls real external LLM REST endpoint when `AI_LLM_API_URL` + `AI_LLM_API_KEY` are configured, falling back to `GroundedAnalyticalExplainer` (`grounded-analyst-v1.0`) for local/CI; always includes date ranges, branches, stale warnings, `requires_human_approval: true`, report links, and `audit_metadata`. | **PASS** |
| **Owner PWA AI Screen Live API Wiring** | `apps/web-owner/src/app/ai/page.tsx` (L36–180) displays `PRESET_ANSWERS` locally rather than calling `POST /api/v1/ai/ask`. | **PARTIAL (Stub)** |

---

### 5. Remaining Builds, Tests & Scale/Outage Scripts (Gates 3–5)

| Target | Command / File | Result |
| --- | --- | --- |
| **Cloud API Gate 3–5 Tests** | `go test -v ./internal/storefront/... ./internal/delivery/... ./internal/telemetry/... ./internal/accounting/... ./internal/hr/... ./internal/reporting/... ./internal/ai/... ./internal/qualification/...` | **PASS** — 19/19 tests `ok` |
| **Web Monorepo Typecheck** | `npm.cmd run typecheck:admin && npm.cmd run typecheck:storefront && npm.cmd run typecheck:owner` | **PASS** — 0 TS errors (and `build:all` passed in Step 30) |
| **Branch Chaos & Outage Drills** | `services/branch-service/internal/chaos/outage_drill_test.go`: 5/5 real SQLite WAL drills PASS (`local_sale_ack_p95 = 4.30ms < 200ms`, `local_kds_p95 = 4.10ms < 1000ms`, 0 lost orders on crash/restart, 0 duplicate effects). | **PASS** |
| **High-Throughput Load CLI** | `services/cloud-api/cmd/loadtest/main.go`: evaluates all 5 release gates (`internal/qualification/gates.go`) with `ci_smoke`, `tenant_steady`, `fleet_scale` presets, but currently feeds synthetic latency samples instead of live HTTP load workers against `-target`. | **PARTIAL (Stub)** |

---

## Executive Gap Summary (Ranked by Release Gate)

| Gate | Severity / Status | Area | Exact File(s) | Shortest Fix Needed |
| --- | --- | --- | --- | --- |
| **Gate 1** | **PARTIAL (Env)** | Local Flutter SDK Verification | `apps/pos-flutter/` | Install Flutter SDK locally (or verify via `.github/workflows/release-flutter.yml` CI run) to execute `flutter analyze` & `flutter test`. |
| **Gate 2** | **PASS (Closed)** | KDS Prep → Recipe Deduction E2E | `services/cloud-api/internal/sync/prep_deduction.go`, `service.go` | Closed: `kds.ticket_updated` (`preparing`) invokes `ApplyKDSPrepDeductionTx` idempotently; verified by `TestKDSPrepConfirmation_LargeOatLatteExtraShotIdempotentReplay`. |
| **Gate 2** | **PASS (Closed)** | Replenishment PO Draft Persistence | `services/cloud-api/internal/procurement/replenishment.go` | Closed: `RunReplenishment` persists PO drafts via `PersistReplenishmentDraftsTx` with deterministic `idempotency_key`; verified by `TestReplenishmentIdempotentDraftPersistence`. |
| **Gate 2** | **PASS (Closed)** | Live OCR Provider Adapter | `services/cloud-api/internal/ocr/http_extractor.go` | Closed: `HTTPVisionExtractor` + `NewExtractorFromEnv()` with sanitization and `httptest` verification. |
| **Gate 3** | **PASS (Closed)** | Live Online Payment Provider | `services/cloud-api/internal/payments/http_gateway.go` | Closed: `HTTPPaymentProvider` + `NewProviderFromEnv()` with authorize/capture/reverse/refund, idempotency headers, HMAC webhooks, and `httptest` verification. |
| **Gate 3** | **PASS (Closed)** | Storefront UI Live API Fetch | `apps/web-storefront/src/app/page.tsx` | Closed: fetches live `/config`, `/menu`, and submits `POST .../checkout` with explicit offline fallback banner. |
| **Gate 4** | **PASS (Closed)** | Owner PWA & AI Analyst Live API Fetch | `apps/web-owner/src/app/page.tsx`, `apps/web-owner/src/app/ai/page.tsx` | Closed: fetches live `/reports/owner/summary`, `/branch-comparison`, and `POST /api/v1/ai/ask` with explicit offline/auth notice. |
| **Gate 5** | **PASS (Closed)** | Live HTTP Load Generation in `cmd/loadtest` | `services/cloud-api/cmd/loadtest/main.go` | Closed: real concurrent `net/http` worker goroutines (`runLiveHTTPWorkers`) with skewed tenant distribution (`-skew`), live default mode, and `httptest` verification. |

---

## Phase 6 — Final Cross-Repo Integration, CI/CD & Release Sign-Off

### Cross-App Contract Matrix — **PASS**
- **`windows_pos` & `android_staff` ↔ `services/branch-service`**:
  - `GET /healthz` → `{ cloud_reachable, pending_outbox_count, branch_id }` parsed into `ConnectionMode.cloudConnected` / `lanOfflineInternet`.
  - `POST /lan/commands/orders` → exact `commands.CreateOrUpdateOrderInput` + `OrderItemInput` (`menu_item_id`, `item_name`, `prep_station`, `quantity`, `unit_price`, `line_total`, `check_number`, `idempotency_key`).
- **`apps/web-kds` ↔ `services/branch-service`**:
  - `GET /lan/kds/tickets` → `{ tickets: [...] }` (always `[]` when empty, zero SQLite `MaxOpenConns(1)` deadlock, verified by `TestKDSTicketsAndHealthz_NoSQLitePoolDeadlock`).
  - `GET /lan/stream` (SSE) + `POST /lan/commands/kds/bump` with visible error banner on failure.
- **`branch-service`, `web-owner`, `web-storefront`, `rider_app` ↔ `services/cloud-api`**:
  - `POST /api/v1/auth/login` (Bearer JWT HS256), `POST /api/v1/branches/{branchId}/heartbeat`, `POST /api/v1/branches/{branchId}/sync/push` (`X-Device-ID` + `X-Device-Signature` + nested savepoints + `pgx.QueryExecModeExec`), `GET /api/v1/reports/owner/{summary,branch-comparison}`, `POST /api/v1/ai/ask`, `GET/POST /api/v1/storefront/{tenantSlug}/{config,menu,checkout}`, `POST /api/v1/delivery/trips/{tripId}/status`, `POST /api/v1/telemetry/gps/batch`.
  - CORS allows `https://*.vercel.app`, `localhost:*`, `127.0.0.1:*`, and configured `CORS_ORIGINS` with `OPTIONS` → `204`.

### CI/CD & Repo Hygiene — **PASS**
- `.github/workflows/ci.yml`: Runs `go build/vet/test` for both Go services and `typecheck:all` + `build:all` for all 4 Next.js apps.
- `.github/workflows/release-flutter.yml`: Uses `actions/setup-java@v5`, scaffolds missing native runners, and builds `windows_pos`, `android_staff`, and `rider_app` with `uses-material-design: true` bundled in all 3 `pubspec.yaml` files.
- `.gitignore`: Blocks `.env`, `*.db`, `.next/`, `build/`, and `start-branch-local.bat` so live secrets and local SQLite files cannot leak into git.
