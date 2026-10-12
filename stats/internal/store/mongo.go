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
	PlayerStatsColl     = "player_stats"
	PlayerGameStatsColl = "player_game_stats"
	RankingsColl        = "rankings"
	StreamStateColl     = "stream_state"

	streamStateID = "history_stream"
	connectTimeout = 10 * time.Second
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
	models := []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "userId", Value: 1}},
			Options: options.Index().SetUnique(true).SetName("uq_player_stats_user"),
		},
		{
			Keys:    bson.D{{Key: "userId", Value: 1}, {Key: "gameType", Value: 1}},
			Options: options.Index().SetUnique(true).SetName("uq_player_game_stats_user_game"),
		},
		{
			Keys:    bson.D{{Key: "period", Value: 1}, {Key: "periodStart", Value: -1}},
			Options: options.Index().SetName("idx_rankings_period"),
		},
	}
	if _, err := s.db.Collection(PlayerStatsColl).Indexes().CreateOne(ctx, models[0]); err != nil {
		return fmt.Errorf("index player_stats: %w", err)
	}
	if _, err := s.db.Collection(PlayerGameStatsColl).Indexes().CreateOne(ctx, models[1]); err != nil {
		return fmt.Errorf("index player_game_stats: %w", err)
	}
	if _, err := s.db.Collection(RankingsColl).Indexes().CreateOne(ctx, models[2]); err != nil {
		return fmt.Errorf("index rankings: %w", err)
	}
	return nil
}