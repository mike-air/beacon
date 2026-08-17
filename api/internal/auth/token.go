// JWT issue/verify. A short signed access token proves identity on every
// request: we sign the user ID into `sub`, set an expiry, and verify the
// signature with the server's secret. No session table, no database lookup —
// the signature is the proof.
//
// Course mapping: Chapter 16 — JWT access tokens. The course also builds
// database-backed refresh-token rotation; we keep a single access token here
// (IssueToken/ParseToken) to stay simple and toolchain-free.

package auth

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const tokenIssuer = "beacon-api"

// ErrInvalidToken is returned for any verify failure — bad signature, wrong
// algorithm, expired, or malformed. The HTTP layer maps it to 401.
var ErrInvalidToken = errors.New("invalid token")

// claims is what we sign into every token. Small on purpose — claims travel on
// every request. Roles live in the database, not here (see Chapter 17).
type claims struct {
	jwt.RegisteredClaims
}

// IssueToken signs a fresh access token for userID, valid for ttl.
func IssueToken(secret string, userID string, ttl time.Duration) (string, error) {
	now := time.Now()
	c := claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    tokenIssuer,
			Subject:   userID,
			ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
		},
	}

	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, c)
	signed, err := tok.SignedString([]byte(secret))
	if err != nil {
		return "", fmt.Errorf("sign token: %w", err)
	}
	return signed, nil
}

// ParseToken verifies the signature and returns the user ID (the `sub` claim).
// The algorithm allow-list plus the keyfunc type-assertion together defend
// against the alg-confusion attacks (alg:none, RS256→HS256 key reuse).
func ParseToken(secret string, raw string) (string, error) {
	keyFunc := func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return []byte(secret), nil
	}

	var c claims
	tok, err := jwt.ParseWithClaims(raw, &c, keyFunc,
		jwt.WithValidMethods([]string{"HS256"}),
		jwt.WithIssuer(tokenIssuer),
		jwt.WithLeeway(30*time.Second),
	)
	if err != nil || !tok.Valid {
		return "", ErrInvalidToken
	}
	if c.Subject == "" {
		return "", ErrInvalidToken
	}
	return c.Subject, nil
}
