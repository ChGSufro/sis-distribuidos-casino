package repository

import (
	"context"

	"github.com/ChGSufro/sis-distribuidos-casino/blackjack/domain"
)

type HandRepository interface {
	CreateHand(ctx context.Context, hand *domain.HandDocument) (*domain.HandDocument, error)
	GetHandByID(ctx context.Context, id string) (*domain.HandDocument, error)
	GetHandByIdempotencyKey(ctx context.Context, idempKey string) (*domain.HandDocument, error)
	UpdateHand(ctx context.Context, hand *domain.HandDocument) error
}

