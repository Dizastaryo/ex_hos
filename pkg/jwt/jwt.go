package jwt

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/seeu/backend/internal/domain"
)

type Manager struct {
	accessSecret   string
	refreshSecret  string
	accessExpHours int
	refreshExpDays int
}

type Claims struct {
	UserID string `json:"user_id"`
	jwt.RegisteredClaims
}

func NewManager(accessSecret, refreshSecret string, accessExpHours, refreshExpDays int) *Manager {
	return &Manager{
		accessSecret:   accessSecret,
		refreshSecret:  refreshSecret,
		accessExpHours: accessExpHours,
		refreshExpDays: refreshExpDays,
	}
}

func (m *Manager) GenerateAccessToken(userID string) (string, error) {
	claims := Claims{
		UserID: userID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Duration(m.accessExpHours) * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    "seeu-api",
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(m.accessSecret))
}

func (m *Manager) GenerateRefreshToken(userID string) (string, error) {
	claims := Claims{
		UserID: userID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Duration(m.refreshExpDays) * 24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    "seeu-api",
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(m.refreshSecret))
}

func (m *Manager) ValidateAccessToken(tokenStr string) (*Claims, error) {
	return m.validateToken(tokenStr, m.accessSecret)
}

func (m *Manager) ValidateRefreshToken(tokenStr string) (*Claims, error) {
	return m.validateToken(tokenStr, m.refreshSecret)
}

func (m *Manager) validateToken(tokenStr, secret string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, domain.ErrTokenInvalid
		}
		return []byte(secret), nil
	})

	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, domain.ErrTokenExpired
		}
		return nil, domain.ErrTokenInvalid
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, domain.ErrTokenInvalid
	}

	return claims, nil
}

func (m *Manager) RefreshExpiry() time.Duration {
	return time.Duration(m.refreshExpDays) * 24 * time.Hour
}
