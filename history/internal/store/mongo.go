package store

import (
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

const (
	EventsCollection = "game_events"
	connectTimeout   = 10 * time.Second
)

type MongoStore struct {
	client *mongo.Client
	db     *mongo.Database
}

func New(ctx context.Context, uri, dbName string) (*MongoStore, error) {
	cctx, cancel := context.WithTimeout(ctx, connectTimeout)
	defer cancel()

	client, err := mongo.Connect(cctx, options.Client().ApplyURI(uri))
	if err != nil {
		return nil, fmt.Errorf("conectar a mongo: %w", err)
	}
	if err := client.Ping(cctx, nil); err != nil {
		_ = client.Disconnect(cctx)
		return nil, fmt.Errorf("ping a mongo: %w", err)
	}

	s := &MongoStore{client: client, db: client.Database(dbName)}
	if err := s.ensureIndexes(ctx); err != nil {
		_ = client.Disconnect(cctx)
		return nil, fmt.Errorf("crear indices: %w", err)
	}
	return s, nil
}

func (s *MongoStore) Close(ctx context.Context) error {
	return s.client.Disconnect(ctx)
}

func (s *MongoStore) ensureIndexes(ctx context.Context) error {
	col := s.db.Collection(EventsCollection)
	models := []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "eventId", Value: 1}},
			Options: options.Index().SetUnique(true),
		},
		{
			Keys: bson.D{{Key: "userId", Value: 1}, {Key: "timestamp", Value: -1}},
		},
		{
			Keys: bson.D{{Key: "gameType", Value: 1}, {Key: "timestamp", Value: -1}},
		},
	}
	if _, err := col.Indexes().CreateMany(ctx, models); err != nil {
		return fmt.Errorf("create indexes: %w", err)
	}
	return nil
}