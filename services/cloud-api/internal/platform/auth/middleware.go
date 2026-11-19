package auth

import (
	"context"
	"net/http"
	"strings"

	"github.com/hospitality-os/cloud-api/internal/platform/db"
	"github.com/hospitality-os/cloud-api/internal/platform/httpx"
	"github.com/jackc/pgx/v5"
)

type contextKey string

const (
	claimsContextKey contextKey = "auth_claims"
	deviceContextKey contextKey = "auth_device"
)

type DevicePrincipal struct {
	DeviceID   string
	TenantID   string
	BranchID   string
	DeviceType string
}

func ClaimsFromContext(ctx context.Context) (*Claims, bool) {
	c, ok := ctx.Value(claimsContextKey).(*Claims)
	return c, ok
}

func DeviceFromContext(ctx context.Context) (*DevicePrincipal, bool) {
	d, ok := ctx.Value(deviceContextKey).(*DevicePrincipal)
	return d, ok
}

type Middleware struct {
	tokens *TokenManager
	pool   *db.Pool
}

func NewMiddleware(tokens *TokenManager, pool *db.Pool) *Middleware {
	return &Middleware{tokens: tokens, pool: pool}
}

// RequireAuth validates a Bearer JWT token and attaches Claims to request context.
func (m *Middleware) RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		header := r.Header.Get("Authorization")
		if !strings.HasPrefix(header, "Bearer ") {
			httpx.WriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Missing or malformed Bearer token", nil)
			return
		}
		tokenStr := strings.TrimSpace(strings.TrimPrefix(header, "Bearer "))
		claims, err := m.tokens.Verify(tokenStr)
		if err != nil {
			httpx.WriteError(w, http.StatusUnauthorized, "INVALID_TOKEN", "Access token is invalid or expired", nil)
			return
		}
		ctx := context.WithValue(r.Context(), claimsContextKey, claims)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// RequireDeviceAuth validates X-Device-ID + X-Device-Signature against the enrolled devices table
// within the caller's tenant RLS transaction and verifies branch scope server-side.
func (m *Middleware) RequireDeviceAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		claims, ok := ClaimsFromContext(r.Context())
		if !ok {
			httpx.WriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required", nil)
			return
		}

		deviceID := strings.TrimSpace(r.Header.Get("X-Device-ID"))
		deviceSig := strings.TrimSpace(r.Header.Get("X-Device-Signature"))
		if deviceID == "" || deviceSig == "" {
			httpx.WriteError(w, http.StatusUnauthorized, "DEVICE_AUTH_REQUIRED", "X-Device-ID and X-Device-Signature headers are required", nil)
			return
		}

		var principal DevicePrincipal
		var storedHash, status string
		err := m.pool.WithTenantTx(r.Context(), claims.TenantID, func(tx pgx.Tx) error {
			return tx.QueryRow(r.Context(), `
				SELECT id, tenant_id, branch_id, device_type, public_key_hash, status
				FROM devices
				WHERE id = $1
			`, deviceID).Scan(&principal.DeviceID, &principal.TenantID, &principal.BranchID, &principal.DeviceType, &storedHash, &status)
		})
		if err != nil || status != "enrolled" || storedHash != HashSecret(claims.TenantID, deviceSig) {
			httpx.WriteError(w, http.StatusUnauthorized, "INVALID_DEVICE", "Device is not enrolled or signature is invalid", nil)
			return
		}

		if branchID := r.PathValue("branchId"); branchID != "" && branchID != principal.BranchID {
			httpx.WriteError(w, http.StatusForbidden, "BRANCH_SCOPE_MISMATCH", "Device is not authorized for this branch", nil)
			return
		}

		ctx := context.WithValue(r.Context(), deviceContextKey, &principal)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// RequireBranchScope ensures the authenticated user is assigned to the requested {branchId} (or is owner).
func (m *Middleware) RequireBranchScope(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		claims, ok := ClaimsFromContext(r.Context())
		if !ok {
			httpx.WriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required", nil)
			return
		}
		if claims.Role == "owner" {
			next.ServeHTTP(w, r)
			return
		}
		branchID := r.PathValue("branchId")
		if branchID == "" {
			next.ServeHTTP(w, r)
			return
		}
		for _, allowed := range claims.BranchIDs {
			if allowed == branchID {
				next.ServeHTTP(w, r)
				return
			}
		}
		httpx.WriteError(w, http.StatusForbidden, "FORBIDDEN_BRANCH", "User does not have access to this branch", nil)
	})
}

// RequireRole restricts access to one of the specified tenant roles.
func (m *Middleware) RequireRole(allowedRoles ...string) func(http.Handler) http.Handler {
	allowed := make(map[string]struct{}, len(allowedRoles))
	for _, r := range allowedRoles {
		allowed[r] = struct{}{}
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims, ok := ClaimsFromContext(r.Context())
			if !ok {
				httpx.WriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required", nil)
				return
			}
			if _, ok := allowed[claims.Role]; !ok {
				httpx.WriteError(w, http.StatusForbidden, "INSUFFICIENT_ROLE", "Role is not permitted for this action", nil)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// RequireEntitlement verifies that the tenant's entitlements include the required module.
func (m *Middleware) RequireEntitlement(module string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims, ok := ClaimsFromContext(r.Context())
			if !ok {
				httpx.WriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required", nil)
				return
			}
			if !claims.Entitlements[module] {
				httpx.WriteError(w, http.StatusForbidden, "MODULE_NOT_ENTITLED", "Tenant is not entitled to module: "+module, nil)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
