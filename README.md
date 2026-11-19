# Hospitality OS — Enterprise Multi-Tenant POS, ERP & Delivery Ecosystem

Production-grade, offline-first Restaurant & Coffee Shop POS, ERP, Inventory, Procurement, Accounting, HR, Delivery, and Executive Owner PWA monorepo.

## Architecture Overview
- **`services/cloud-api`**: Go modular monolith (PostgreSQL + RLS `pos_app_user`, `shopspring/decimal`, Auth/RBAC, Inventory, Procurement + OCR + 3-Way Match, Double-Entry Accounting, HR, Delivery, Telemetry, Reporting, Read-Only AI Agent, Qualification Load Harness).
- **`services/branch-service`**: Pure-Go SQLite WAL (`modernc.org/sqlite`) local LAN server with atomic transactions, SSE KDS hub, FIFO outbox replay, and durable ESC/POS print spooler.
- **`apps/web-admin`**: Next.js 15 Back-Office ERP (17 prerendered operational & financial screens).
- **`apps/web-storefront`**: Next.js 15 3-theme installable Customer Ordering PWA + live rider tracking.
- **`apps/web-kds`**: Next.js 15 real-time Kitchen Display System (SSE + 3s polling fallback).
- **`apps/web-owner`**: Next.js 15 Executive Owner PWA + Read-Only AI Reporting Analyst.
- **`apps/pos-flutter`**: Flutter monorepo (`pos_domain`, `windows_pos`, `android_staff`, `rider_app`).

---

## Production Deployment & Pilot Go-Live Runbook

### 1. Push Monorepo to GitHub
```bash
git remote add origin https://github.com/<your-org>/<your-repo>.git
git branch -M main
git push -u origin main
```

### 2. Deploy 4 Independent Next.js Apps on Vercel (1 Repo)
Create 4 Vercel projects connected to the same GitHub repository, each with its own **Root Directory**:

| Vercel Project | Root Directory | Build Command (via `vercel.json`) | Required Env Vars |
| --- | --- | --- | --- |
| `hospitality-admin` | `apps/web-admin` | `cd ../.. && npm run build:admin` | `NEXT_PUBLIC_CLOUD_API_URL` |
| `hospitality-storefront` | `apps/web-storefront` | `cd ../.. && npm run build:storefront` | `NEXT_PUBLIC_CLOUD_API_URL`, `NEXT_PUBLIC_DEFAULT_TENANT_SLUG` |
| `hospitality-kds` | `apps/web-kds` | `cd ../.. && npm run build:kds` | `NEXT_PUBLIC_BRANCH_LAN_URL`, `NEXT_PUBLIC_CLOUD_API_URL` |
| `hospitality-owner` | `apps/web-owner` | `cd ../.. && npm run build:owner` | `NEXT_PUBLIC_CLOUD_API_URL` |

### 3. Deploy Cloud Backend & Managed PostgreSQL
```bash
cp infra/.env.example infra/.env
# Edit infra/.env with production DATABASE_URL, APP_DATABASE_URL, JWT_SECRET, S3_*

docker compose -f infra/docker-compose.prod.yml up -d --build

# Run PostgreSQL migrations & pilot seed
docker compose -f infra/docker-compose.prod.yml exec cloud-api /app/migrate
docker compose -f infra/docker-compose.prod.yml exec cloud-api /app/seed
```

### 4. Local Branch PC Setup (`services/branch-service`)
- **Docker**: `docker compose -f infra/docker-compose.prod.yml up -d branch-service`
- **Windows Service (NSSM)**:
  ```powershell
  nssm install HospitalityBranchService "C:\HospitalityOS\branch.exe"
  nssm set HospitalityBranchService AppEnvironmentExtra BRANCH_DB_PATH=C:\ProgramData\HospitalityOS\branch.db CLOUD_API_URL=https://api.example.com
  nssm start HospitalityBranchService
  ```
- Enroll POS, KDS, and Android Staff terminals via `POST /api/v1/devices/enroll`.

### 5. Flutter Release Builds
```bash
# Windows POS (.exe)
cd apps/pos-flutter/apps/windows_pos && flutter build windows --release

# Android Staff & Rider APKs
cd apps/pos-flutter/apps/android_staff && flutter build apk --release
cd apps/pos-flutter/apps/rider_app && flutter build apk --release
```
Or tag a release (`git tag v1.0.0 && git push --tags`) to trigger `.github/workflows/release-flutter.yml`.
