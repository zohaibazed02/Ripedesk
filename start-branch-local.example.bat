@echo off
title Hospitality OS - Branch Service (LAN Port 8090)
echo Starting Hospitality OS Branch Service on http://127.0.0.1:8090 ...
set BRANCH_SERVICE_ADDR=:8090
set TENANT_ID=422200de-8670-4aa8-a9cf-b09ec4001ebd
set BRANCH_ID=2c905f5a-8dde-448b-a3fa-592c0de66c15
set CLOUD_API_URL=https://ripedesk-cloud-api.onrender.com
set BRANCH_AUTH_TOKEN=PASTE_JWT_FROM_GEN_BRANCH_TOKEN
set BRANCH_DEVICE_ID=PASTE_DEVICE_ID_FROM_ENROLL
set BRANCH_DEVICE_SECRET=PASTE_DEVICE_SECRET_FROM_ENROLL
cd /d "%~dp0services\branch-service"
go run ./cmd/branch
pause
