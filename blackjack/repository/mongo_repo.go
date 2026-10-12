package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/ChGSufro/sis-distribuidos-casino/blackjack/domain"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

type MongoHandRepository struct {
	coll *mongo.Collection
}

func NewMongoHandRepository(db *mongo.Database) *MongoHandRepository {
	return &MongoHandRepository{
		coll: db.Collection("hands"),
	}
}

func (r *MongoHandRepository) CreateHand(ctx context.Context, hand *domain.HandDocument) (*domain.HandDocument, error) {
	if hand.ID.IsZero() {
		hand.ID = bson.NewObjectID()
	}

	res, err := r.coll.InsertOne(ctx, hand)
	if err != nil {
		return nil, fmt.Errorf("insertar mano en mongodb: %w", err)
	}

	if oid, ok := res.InsertedID.(bson.ObjectID); ok {
		hand.ID = oid
	}

	return hand, nil
}

func (r *MongoHandRepository) GetHandByID(ctx context.Context, id string) (*domain.HandDocument, error) {
	oid, err := bson.ObjectIDFromHex(id)
	if err != nil {
		return nil, domain.ErrHandNotFound
	}

	var hand domain.HandDocument
	err = r.coll.FindOne(ctx, bson.M{"_id": oid}).Decode(&hand)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, domain.ErrHandNotFound
		}
		return nil, fmt.Errorf("obtener mano por id: %w", err)
	}

	return &hand, nil
}

func (r *MongoHandRepository) GetHandByIdempotencyKey(ctx context.Context, idempKey string) (*domain.HandDocument, error) {
	var hand domain.HandDocument
	err := r.coll.FindOne(ctx, bson.M{"idempotencyKey": idempKey}).Decode(&hand)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, domain.ErrHandNotFound
		}
		return nil, fmt.Errorf("obtener mano por clave de idempotencia: %w", err)
	}

	return &hand, nil
}

func (r *MongoHandRepository) UpdateHand(ctx context.Context, hand *domain.HandDocument) error {
	filter := bson.M{"_id": hand.ID}
	_, err := r.coll.ReplaceOne(ctx, filter, hand)
	if err != nil {
		return fmt.Errorf("actualizar mano en mongodb: %w", err)
	}
	return nil
}

