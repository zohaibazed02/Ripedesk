package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	log.Println("cloud-worker started: listening for sync reconciliation and ERP ledger jobs")
	for {
		select {
		case <-ctx.Done():
			log.Println("cloud-worker shutting down cleanly")
			return
		case ts := <-ticker.C:
			log.Printf("cloud-worker heartbeat at %s", ts.UTC().Format(time.RFC3339))
		}
	}
}
