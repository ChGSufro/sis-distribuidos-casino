package domain

import (
	"time"

	"github.com/google/uuid"
)

type HoldStatus string

const (
	HoldStatusActive   HoldStatus = "active"
	HoldStatusSettled  HoldStatus = "settled"
	HoldStatusReleased HoldStatus = "released"
)

type TransactionType string

const (
	TxTypeInit    TransactionType = "init"
	TxTypeDeposit TransactionType = "deposit"
	TxTypeHold    TransactionType = "hold"
	TxTypeSettle  TransactionType = "settle"
	TxTypeRelease TransactionType = "release"
	TxTypeBuyIn   TransactionType = "buyin"
	TxTypePayout  TransactionType = "payout"
)

// Wallet representa la billetera de un usuario en el libro contable.
type Wallet struct {
	ID           uuid.UUID `json:"id"`
	UserID       uuid.UUID `json:"user_id"`
	BalanceCents int64     `json:"balance_cents"`
	Version      int32     `json:"version"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// Hold representa una reserva de fondos para una apuesta o compra en juego.
type Hold struct {
	ID             uuid.UUID  `json:"id"`
	WalletID       uuid.UUID  `json:"wallet_id"`
	AmountCents    int64      `json:"amount_cents"`
	GameType       string     `json:"game_type"`
	GameRefID      string     `json:"game_ref_id"`
	Status         HoldStatus `json:"status"`
	IdempotencyKey string     `json:"idempotency_key"`
	CreatedAt      time.Time  `json:"created_at"`
	ResolvedAt     *time.Time `json:"resolved_at,omitempty"`
}

// Transaction registra cualquier movimiento que afecte el balance de una billetera.
type Transaction struct {
	ID                uuid.UUID       `json:"id"`
	WalletID          uuid.UUID       `json:"wallet_id"`
	HoldID            *uuid.UUID      `json:"hold_id,omitempty"`
	Type              TransactionType `json:"type"`
	AmountCents       int64           `json:"amount_cents"`
	BalanceAfterCents int64           `json:"balance_after_cents"`
	CreatedAt         time.Time       `json:"created_at"`
}

