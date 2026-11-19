@echo off
title Hospitality OS - Branch Service (LAN Port 8090)
echo Starting Hospitality OS Branch Service on http://127.0.0.1:8090 ...
set BRANCH_SERVICE_ADDR=:8090
set CLOUD_API_URL=https://ripedesk-cloud-api.onrender.com
cd /d "%~dp0services\branch-service"
go run ./cmd/branch
pause
