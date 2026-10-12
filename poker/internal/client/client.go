// Package client define los limites (puertos) del servicio de Poker hacia
// otros servicios del sistema. El motor de Poker solo conoce estas interfaces,
// de modo que la integracion real (gRPC contra Wallet e History) pueda
// agregarse despues sin tocar la logica de negocio.
//
// Estado actual: el servicio usa las implementaciones Noop*, que permiten
// operar de forma autonoma mientras los servicios Wallet/History no esten
// disponibles. Para conectar de verdad, basta con proveer una implementacion
// de estas interfaces (ver wallet_grpc.go.example / history_grpc.go.example).
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
	// Settle liquida un hold devolviendo winAmountCents al usuario.
	Settle(ctx context.Context, holdID string, winAmountCents int64) (int64, error)
}

// HistoryClient abstrae el History & Audit Service.
type HistoryClient interface {
	// RecordGameEvent registra el resumen de una partida finalizada.
	RecordGameEvent(ctx context.Context, event GameEvent) error
}

// GameEvent es el resumen de una ronda finalizada que se envia a History.
type GameEvent struct {
	GameType   string
	TableID    string
	RoundID    string
	PotCents   int64
	Winners    []WinnerInfo
	FinishedAt time.Time
}

// WinnerInfo identifica a un ganador dentro de un GameEvent.
type WinnerInfo struct {
	UserID      string
	AmountCents int64
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

// NoopHistory es una implementacion de HistoryClient que descarta los eventos.
type NoopHistory struct{}

// RecordGameEvent no hace nada.
func (NoopHistory) RecordGameEvent(ctx context.Context, event GameEvent) error { return nil }
