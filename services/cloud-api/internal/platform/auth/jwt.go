package auth

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type Claims struct {
	UserID       string          `json:"user_id"`
	TenantID     string          `json:"tenant_id"`
	Role         string          `json:"role"`
	BranchIDs    []string        `json:"branch_ids"`
	CanOverride  bool            `json:"can_override"`
	Entitlements map[string]bool `json:"entitlements,omitempty"`
	jwt.RegisteredClaims
}

type TokenManager struct {
	secret []byte
	ttl    time.Duration
}

func NewTokenManager(secret string, ttl time.Duration) *TokenManager {
	if ttl <= 0 {
		ttl = 12 * time.Hour
	}
	return &TokenManager{
		secret: []byte(secret),
		ttl:    ttl,
	}
}

func (tm *TokenManager) Sign(claims Claims) (string, error) {
	now := time.Now().UTC()
	claims.RegisteredClaims = jwt.RegisteredClaims{
		IssuedAt:  jwt.NewNumericDate(now),
		ExpiresAt: jwt.NewNumericDate(now.Add(tm.ttl)),
		Issuer:    "hospitality-os-cloud-api",
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(tm.secret)
}

func (tm *TokenManager) Verify(tokenStr string) (*Claims, error) {
	parsed, err := jwt.ParseWithClaims(tokenStr, &Claims{}, func(token *jwt.Token) (any, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return tm.secret, nil
	})
	if err != nil {
		return nil, err
	}
	claims, ok := parsed.Claims.(*Claims)
	if !ok || !parsed.Valid {
		return nil, errors.New("invalid token claims")
	}
	return claims, nil
}

func HashSecret(salt, raw string) string {
	sum := sha256.Sum256([]byte(salt + ":" + raw))
	return hex.EncodeToString(sum[:])
}
