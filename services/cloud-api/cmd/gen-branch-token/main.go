package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/hospitality-os/cloud-api/internal/platform/auth"
)

func main() {
	secret := flag.String("secret", os.Getenv("JWT_SECRET"), "JWT_SECRET used by cloud-api")
	tenantID := flag.String("tenant", os.Getenv("TENANT_ID"), "Tenant UUID")
	branchID := flag.String("branch", os.Getenv("BRANCH_ID"), "Branch UUID from PostgreSQL branches table")
	userID := flag.String("user", "branch-service-agent", "Actor user_id claim")
	role := flag.String("role", "branch_manager", "Role claim (owner | branch_manager)")
	days := flag.Int("days", 365, "Token validity in days")
	flag.Parse()

	if *secret == "" || *tenantID == "" || *branchID == "" {
		fmt.Fprintln(os.Stderr, "Usage: go run ./cmd/gen-branch-token -secret=<JWT_SECRET> -tenant=<TENANT_UUID> -branch=<BRANCH_UUID>")
		fmt.Fprintln(os.Stderr, "Or set JWT_SECRET, TENANT_ID, and BRANCH_ID environment variables.")
		os.Exit(2)
	}

	tm := auth.NewTokenManager(*secret, time.Duration(*days)*24*time.Hour)
	token, err := tm.Sign(auth.Claims{
		UserID:      *userID,
		TenantID:    *tenantID,
		Role:        *role,
		BranchIDs:   []string{*branchID},
		CanOverride: false,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to sign token: %v\n", err)
		os.Exit(1)
	}

	fmt.Println(token)
}
