# syntax=docker/dockerfile:1
FROM golang:1.23-alpine AS builder
WORKDIR /src
RUN apk add --no-cache ca-certificates

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/branch ./cmd/branch

FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata wget \
 && adduser -D -u 10002 branchuser \
 && mkdir -p /var/lib/branch-service \
 && chown -R branchuser:branchuser /var/lib/branch-service
WORKDIR /app

COPY --from=builder /out/branch /app/branch
RUN chown branchuser:branchuser /app/branch

USER branchuser
VOLUME ["/var/lib/branch-service"]
EXPOSE 8090
HEALTHCHECK --interval=20s --timeout=3s --start-period=5s --retries=3 \
  CMD wget -qO- http://127.0.0.1:8090/lan/health || exit 1

ENTRYPOINT ["/app/branch"]
