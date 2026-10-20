// Package client define los limites (puertos) del servicio de Poker hacia
// otros servicios del sistema. El motor de Poker solo conoce estas interfaces,
// de modo que la integracion real (gRPC contra Wallet e History) pueda
// agregarse sin tocar la logica de negocio.
//
// Implementaciones reales: wallet_grpc.go (Wallet) e history_grpc.go (History).
// Los Noop* permiten seguir operando de forma autonoma si los servicios
// externos no estan disponibles.
package client

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// WalletClient abstrae el Wallet Service.
type WalletClient interface {
	// Hold reserva fondos del usuario como buy-in y devuelve el holdId.
	Hold(ctx context.Context, userID string, amountCents int64, gameType, gameRefID, idempotencyKey string) (string, error)
	// Settle liquida un hold acreditando winAmountCents al usuario.
	Settle(ctx context.Context, holdID string, winAmountCents int64) (int64, error)
	// Release cancela un hold sin acreditar nada (compensacion de fallos).
	Release(ctx context.Context, holdID string) error
	// Close cierra la conexion subyacente.
	Close() error
}

// HistoryClient abstrae el History & Audit Service.
type HistoryClient interface {
	// RecordGameEvent registra un evento por jugador (compra, resultado, ...).
	RecordGameEvent(ctx context.Context, event GameEvent) error
	// Close cierra la conexion subyacente.
	Close() error
}

// GameEvent es un evento por jugador enviado a History. Se corresponde con
// el mensaje GameEvent del proto de History (history/pb/history.proto):
// EventID es la clave de idempotencia, Details es JSON con la accion.
type GameEvent struct {
	EventID   string
	UserID    string
	GameType  string
	GameRefID string
	Action    string
	Details   string
	Timestamp time.Time
}

// NoopWallet es una implementacion de WalletClient que no contacta al Wallet
// Service: genera holdIds locales. Util para operar sin el servicio externo.
type NoopWallet struct{}

// Hold devuelve un holdId sintetico.
func (NoopWallet) Hold(ctx context.Context, userID string, amountCents int64, gameType, gameRefID, idempotencyKey string) (string, error) {
	return "local-" + primitive.NewObjectID().Hex(), nil
}

// Settle confirma la liquidacion devolviendo el monto tal cual.
func (NoopWallet) Settle(ctx context.Context, holdID string, winAmountCents int64) (int64, error) {
	return winAmountCents, nil
}

// Release no hace nada.
func (NoopWallet) Release(ctx context.Context, holdID string) error { return nil }

// Close no hace nada.
func (NoopWallet) Close() error { return nil }

// NoopHistory es una implementacion de HistoryClient que descarta los eventos.
type NoopHistory struct{}

// RecordGameEvent no hace nada.
func (NoopHistory) RecordGameEvent(ctx context.Context, event GameEvent) error { return nil }

// Close no hace nada.
func (NoopHistory) Close() error { return nil }
