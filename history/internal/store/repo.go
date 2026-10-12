package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type EventRecord struct {
	ID        primitive.ObjectID `bson:"_id,omitempty"`
	EventID   string             `bson:"eventId"`
	UserID    string             `bson:"userId"`
	GameType  string             `bson:"gameType"`
	GameRefID string             `bson:"gameRefId"`
	Action    string             `bson:"action"`
	Details   string             `bson:"details"`
	Timestamp time.Time          `bson:"timestamp"`
}

// InsertEvent inserta un evento (append-only). Si ya existe un evento con el
// mismo eventId devuelve exists=true sin duplicar el documento (idempotencia).
func (s *MongoStore) InsertEvent(ctx context.Context, ev *EventRecord) (exists bool, err error) {
	if _, err := s.db.Collection(EventsCollection).InsertOne(ctx, ev); err != nil {
		if mongo.IsDuplicateKeyError(err) {
			return true, nil
		}
		return false, fmt.Errorf("insert event: %w", err)
	}
	return false, nil
}

func (s *MongoStore) FindByEventID(ctx context.Context, eventID string) (*EventRecord, error) {
	var rec EventRecord
	err := s.db.Collection(EventsCollection).FindOne(ctx, bson.M{"eventId": eventID}).Decode(&rec)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("find event: %w", err)
	}
	return &rec, nil
}

func (s *MongoStore) QueryHistory(ctx context.Context, userID, gameType string, skip, limit int64) ([]EventRecord, error) {
	filter := bson.M{"userId": userID}
	if gameType != "" {
		filter["gameType"] = gameType
	}
	opts := options.Find().
		SetSkip(skip).
		SetLimit(limit).
		SetSort(bson.D{{Key: "timestamp", Value: -1}, {Key: "_id", Value: -1}})

	cur, err := s.db.Collection(EventsCollection).Find(ctx, filter, opts)
	if err != nil {
		return nil, fmt.Errorf("query history: %w", err)
	}
	defer cur.Close(ctx)

	var events []EventRecord
	if err := cur.All(ctx, &events); err != nil {
		return nil, fmt.Errorf("decode history: %w", err)
	}
	return events, nil
}

func (s *MongoStore) CountEvents(ctx context.Context, userID, gameType string) (int64, error) {
	filter := bson.M{"userId": userID}
	if gameType != "" {
		filter["gameType"] = gameType
	}
	count, err := s.db.Collection(EventsCollection).CountDocuments(ctx, filter)
	if err != nil {
		return 0, fmt.Errorf("count history: %w", err)
	}
	return count, nil
}

// EventsAfterCursor devuelve hasta limit eventos ordenados por _id ascendente
// y posteriores al cursor (ObjectId hex). cursor vacio = desde el inicio.
func (s *MongoStore) EventsAfterCursor(ctx context.Context, cursor string, limit int64) ([]EventRecord, error) {
	filter := bson.M{}
	if cursor != "" {
		oid, err := primitive.ObjectIDFromHex(cursor)
		if err != nil {
			return nil, fmt.Errorf("invalid cursor: %w", err)
		}
		filter["_id"] = bson.M{"$gt": oid}
	}
	opts := options.Find().
		SetSort(bson.D{{Key: "_id", Value: 1}}).
		SetLimit(limit)

	cur, err := s.db.Collection(EventsCollection).Find(ctx, filter, opts)
	if err != nil {
		return nil, fmt.Errorf("query after cursor: %w", err)
	}
	defer cur.Close(ctx)

	var events []EventRecord
	if err := cur.All(ctx, &events); err != nil {
		return nil, fmt.Errorf("decode after cursor: %w", err)
	}
	return events, nil
}