package auth

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// Duraciones de los tokens segun AGENTS.md 17.1: access corto (15 min)
// y refresh largo (7 dias).
const (
	AccessTokenTTL  = 15 * time.Minute
	RefreshTokenTTL = 7 * 24 * time.Hour
)

// Claims es la identidad que viaja en los tokens. sub es el id del
// admin; username es informativo para el panel.
type Claims struct {
	Username string `json:"username"`
	jwt.RegisteredClaims
}

// TokenManager firma y valida tokens JWT (HS256). Es la unica capa que
// toca JWT; el secret nunca se loguea.
type TokenManager struct {
	secret []byte
	now    func() time.Time // inyectable para tests
}

// NewTokenManager construye el manager con el secret (JWT_SECRET).
func NewTokenManager(secret string) *TokenManager {
	return &TokenManager{
		secret: []byte(secret),
		now:    time.Now,
	}
}

// IssueAccess genera el access token para el admin (exp 15 min).
func (m *TokenManager) IssueAccess(a Admin) (string, error) {
	return m.issue(a, AccessTokenTTL)
}

// IssueRefresh genera el refresh token para el admin (exp 7 dias).
// Stateless: sin sesiones en DB (AGENTS.md 17.1).
func (m *TokenManager) IssueRefresh(a Admin) (string, error) {
	return m.issue(a, RefreshTokenTTL)
}

func (m *TokenManager) issue(a Admin, ttl time.Duration) (string, error) {
	now := m.now()
	claims := Claims{
		Username: a.Username,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   fmt.Sprintf("%d", a.ID),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(m.secret)
}

// ParseAccess valida un access token y devuelve sus claims.
func (m *TokenManager) ParseAccess(raw string) (*Claims, error) {
	return m.parse(raw)
}

// ParseRefresh valida un refresh token y devuelve sus claims.
func (m *TokenManager) ParseRefresh(raw string) (*Claims, error) {
	return m.parse(raw)
}

func (m *TokenManager) parse(raw string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(raw, &Claims{}, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("auth: unexpected signing method %v", t.Header["alg"])
		}
		return m.secret, nil
	})
	if err != nil {
		return nil, fmt.Errorf("auth: parse token: %w", err)
	}
	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, errors.New("auth: invalid token claims")
	}
	return claims, nil
}
