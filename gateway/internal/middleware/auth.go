package middleware

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	pb_users "github.com/ChGSufro/sis-distribuidos-casino/gateway/pb/users"
)

type contextKey string

const (
	userIDKey   contextKey = "userID"
	usernameKey contextKey = "username"
	tokenKey    contextKey = "token"
)

type AuthMiddleware struct {
	usersClient pb_users.UsersServiceClient
}

func NewAuthMiddleware(usersClient pb_users.UsersServiceClient) *AuthMiddleware {
	return &AuthMiddleware{usersClient: usersClient}
}

func (a *AuthMiddleware) RequireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			writeAuthError(w, http.StatusUnauthorized, "cabecera Authorization requerida")
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			writeAuthError(w, http.StatusUnauthorized, "formato de cabecera inválido, se requiere 'Bearer <token>'")
			return
		}

		token := strings.TrimSpace(parts[1])
		if token == "" {
			writeAuthError(w, http.StatusUnauthorized, "token vacío")
			return
		}

		// Validar token contra UsersService gRPC
		resp, err := a.usersClient.ValidateToken(r.Context(), &pb_users.ValidateTokenRequest{
			AccessToken: token,
		})
		if err != nil {
			writeAuthError(w, http.StatusServiceUnavailable, "servicio de autenticación no disponible")
			return
		}
		if !resp.GetValid() {
			writeAuthError(w, http.StatusUnauthorized, "token inválido, revocado o expirado")
			return
		}

		// Inyectar datos del usuario validado en el contexto
		ctx := context.WithValue(r.Context(), userIDKey, resp.GetUserId())
		ctx = context.WithValue(ctx, usernameKey, resp.GetUsername())
		ctx = context.WithValue(ctx, tokenKey, token)

		next.ServeHTTP(w, r.WithContext(ctx))
	}
}

func UserIDFromContext(ctx context.Context) string {
	if val, ok := ctx.Value(userIDKey).(string); ok {
		return val
	}
	return ""
}

func UsernameFromContext(ctx context.Context) string {
	if val, ok := ctx.Value(usernameKey).(string); ok {
		return val
	}
	return ""
}

func TokenFromContext(ctx context.Context) string {
	if val, ok := ctx.Value(tokenKey).(string); ok {
		return val
	}
	return ""
}

func writeAuthError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"error":   msg,
		"status":  status,
		"success": false,
	})
}
