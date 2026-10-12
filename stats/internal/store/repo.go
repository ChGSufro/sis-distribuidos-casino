package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/ChGSufro/sis-distribuidos-casino/stats/internal/agg"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// ── player_stats ────────────────────────────────────────────────────────────

func (s *MongoStore) GetPlayerStats(ctx context.Context, userID string) (*agg.PlayerStatsDoc, error) {
	var doc agg.PlayerStatsDoc
	err := s.db.Collection(PlayerStatsColl).FindOne(ctx, bson.M{"userId": userID}).Decode(&doc)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get player stats: %w", err)
	}
	return &doc, nil
}

// UpsertPlayerStats reemplaza el doc (escritor único: solo el consumidor del stream).
func (s *MongoStore) UpsertPlayerStats(ctx context.Context, doc *agg.PlayerStatsDoc) error {
	_, err := s.db.Collection(PlayerStatsColl).ReplaceOne(
		ctx,
		bson.M{"userId": doc.UserID},
		doc,
		options.Replace().SetUpsert(true),
	)
	if err != nil {
		return fmt.Errorf("upsert player stats: %w", err)
	}
	return nil
}

func (s *MongoStore) AllPlayerStats(ctx context.Context) ([]agg.PlayerStatsDoc, error) {
	cur, err := s.db.Collection(PlayerStatsColl).Find(ctx, bson.M{})
	if err != nil {
		return nil, fmt.Errorf("query all player stats: %w", err)
	}
	defer cur.Close(ctx)

	var docs []agg.PlayerStatsDoc
	if err := cur.All(ctx, &docs); err != nil {
		return nil, fmt.Errorf("decode all player stats: %w", err)
	}
	return docs, nil
}

// ── player_game_stats ───────────────────────────────────────────────────────

type PlayerGameStatsDoc struct {
	UserID            string    `bson:"userId"`
	GameType          string    `bson:"gameType"`
	GamesPlayed       int       `bson:"gamesPlayed"`
	TotalWageredCents int64     `bson:"totalWageredCents"`
	TotalWonCents     int64     `bson:"totalWonCents"`
	LargestWinCents   int64     `bson:"largestWinCents"`
	LastUpdated       time.Time `bson:"lastUpdated"`
}

func (s *MongoStore) GetPlayerGameStats(ctx context.Context, userID, gameType string) (*PlayerGameStatsDoc, error) {
	var doc PlayerGameStatsDoc
	err := s.db.Collection(PlayerGameStatsColl).
		FindOne(ctx, bson.M{"userId": userID, "gameType": gameType}).Decode(&doc)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get player game stats: %w", err)
	}
	return &doc, nil
}

func (s *MongoStore) ApplyPlayerGameStats(ctx context.Context, userID, gameType string, m agg.Metrics) error {
	played := int64(0)
	if m.Played {
		played = 1
	}
	update := bson.D{
		{Key: "$inc", Value: bson.D{
			{Key: "gamesPlayed", Value: played},
			{Key: "totalWageredCents", Value: m.WageredCents},
			{Key: "totalWonCents", Value: m.WonCents},
		}},
		{Key: "$max", Value: bson.D{{Key: "largestWinCents", Value: m.WonCents}}},
		{Key: "$set", Value: bson.D{{Key: "lastUpdated", Value: time.Now().UTC()}}},
		{Key: "$setOnInsert", Value: bson.D{{Key: "userId", Value: userID}, {Key: "gameType", Value: gameType}}},
	}
	_, err := s.db.Collection(PlayerGameStatsColl).UpdateOne(
		ctx,
		bson.M{"userId": userID, "gameType": gameType},
		update,
		options.Update().SetUpsert(true),
	)
	if err != nil {
		return fmt.Errorf("apply player game stats: %w", err)
	}
	return nil
}

// ── stream_state (cursor de reconexión) ─────────────────────────────────────

func (s *MongoStore) LoadStreamCursor(ctx context.Context) (string, error) {
	var out struct {
		LastCursor string `bson:"lastCursor"`
	}
	err := s.db.Collection(StreamStateColl).FindOne(ctx, bson.M{"_id": streamStateID}).Decode(&out)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("load stream cursor: %w", err)
	}
	return out.LastCursor, nil
}

func (s *MongoStore) SaveStreamCursor(ctx context.Context, cursor string) error {
	update := bson.D{
		{Key: "$set", Value: bson.D{
			{Key: "lastCursor", Value: cursor},
			{Key: "updatedAt", Value: time.Now().UTC()},
		}},
	}
	_, err := s.db.Collection(StreamStateColl).UpdateOne(
		ctx,
		bson.M{"_id": streamStateID},
		update,
		options.Update().SetUpsert(true),
	)
	if err != nil {
		return fmt.Errorf("save stream cursor: %w", err)
	}
	return nil
}

// ── rankings ─────────────────────────────────────────────────────────────────

type RankingEntry struct {
	UserID      string `bson:"userId"`
	Rank        int    `bson:"rank"`
	ScoreCents  int64  `bson:"scoreCents"`
	GamesPlayed int    `bson:"gamesPlayed"`
}

type RankingDoc struct {
	Period      string         `bson:"period"`
	PeriodStart *time.Time     `bson:"periodStart,omitempty"`
	Entries     []RankingEntry `bson:"entries"`
	ComputedAt  time.Time      `bson:"computedAt"`
}

func (s *MongoStore) SaveRanking(ctx context.Context, doc *RankingDoc) error {
	if _, err := s.db.Collection(RankingsColl).InsertOne(ctx, doc); err != nil {
		return fmt.Errorf("save ranking: %w", err)
	}
	return nil
}