package domain

import "errors"

var (
	ErrWalletNotFound      = errors.New("wallet not found")
	ErrWalletAlreadyExists = errors.New("wallet already exists")
	ErrInsufficientFunds   = errors.New("insufficient funds")
	ErrHoldNotFound        = errors.New("hold not found")
	ErrHoldAlreadyResolved = errors.New("hold is already resolved")
	ErrInvalidAmount       = errors.New("amount must be greater than zero")
	ErrInvalidWinAmount    = errors.New("win amount cannot be negative")
	ErrInvalidUUID         = errors.New("invalid uuid format")
	ErrEmptyGameRef        = errors.New("game reference id cannot be empty")
	ErrEmptyIdempotencyKey = errors.New("idempotency key cannot be empty")
	ErrOptimisticLock      = errors.New("concurrent modification detected")
)

