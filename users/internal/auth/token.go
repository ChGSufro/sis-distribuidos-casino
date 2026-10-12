package auth

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// ErrInvalidToken indica que un token no es valido o expiro.
var ErrInvalidToken = errors.New("token invalido")

const refreshAudience = "refresh"

// TokenManager emite y valida los JWT del servicio.
type TokenManager struct {
	secret     []byte
	accessTTL  time.Duration
	refreshTTL time.Duration
}

// AccessClaims son los claims del access token.
type AccessClaims struct {
	Username string `json:"username"`
	jwt.RegisteredClaims
}

// NewTokenManager construye un emisor de tokens con la clave y TTLs dados.
func NewTokenManager(secret string, accessTTL, refreshTTL time.Duration) *TokenManager {
	return &TokenManager{
		secret:     []byte(secret),
		accessTTL:  accessTTL,
		refreshTTL: refreshTTL,
	}
}

// GenerateAccess emite un access token para el usuario y devuelve su expiracion.
func (m *TokenManager) GenerateAccess(userID, username string) (string, time.Time, error) {
	now := time.Now()
	exp := now.Add(m.accessTTL)
	claims := AccessClaims{
		Username: username,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(exp),
		},
	}
	return m.sign(claims), exp, nil
}

// GenerateRefresh emite un refresh token para el usuario y devuelve su expiracion.
func (m *TokenManager) GenerateRefresh(userID string) (string, time.Time, error) {
	now := time.Now()
	exp := now.Add(m.refreshTTL)
	claims := jwt.RegisteredClaims{
		Subject:   userID,
		IssuedAt:  jwt.NewNumericDate(now),
		ExpiresAt: jwt.NewNumericDate(exp),
		Audience:  jwt.ClaimStrings{refreshAudience},
	}
	return m.sign(claims), exp, nil
}

// ParseAccess valida un access token y devuelve sus claims.
func (m *TokenManager) ParseAccess(tokenStr string) (*AccessClaims, error) {
	claims := &AccessClaims{}
	if err := m.parse(tokenStr, claims); err != nil {
		return nil, err
	}
	return claims, nil
}

// ParseRefresh valida un refresh token y devuelve el userID (subject).
func (m *TokenManager) ParseRefresh(tokenStr string) (string, error) {
	claims := &jwt.RegisteredClaims{}
	if err := m.parse(tokenStr, claims, jwt.WithAudience(refreshAudience)); err != nil {
		return "", err
	}
	if claims.Subject == "" {
		return "", ErrInvalidToken
	}
	return claims.Subject, nil
}

// HashToken devuelve el SHA-256 hexadecimal de un token para persistirlo.
func HashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func (m *TokenManager) sign(claims jwt.Claims) string {
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	s, _ := tok.SignedString(m.secret)
	return s
}

func (m *TokenManager) parse(tokenStr string, claims jwt.Claims, opts ...jwt.ParserOption) error {
	opts = append(opts, jwt.WithValidMethods([]string{"HS256"}))
	tok, err := jwt.ParseWithClaims(tokenStr, claims, func(*jwt.Token) (interface{}, error) {
		return m.secret, nil
	}, opts...)
	if err != nil || !tok.Valid {
		return ErrInvalidToken
	}
	return nil
}
