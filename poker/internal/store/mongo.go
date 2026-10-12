// Package store implementa la persistencia del servicio de Poker en MongoDB
// (base de datos poker_state_db, colecciones tables y rounds).
package store

import (
	"context"
	"errors"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// ErrNotFound indica que el documento solicitado no existe.
var ErrNotFound = errors.New("documento no encontrado")

// Mongo agrupa las colecciones de poker_state_db.
type Mongo struct {
	client  *mongo.Client
	tables  *mongo.Collection
	rounds  *mongo.Collection
}

// New conecta a MongoDB y garantiza los indices del modelo.
func New(ctx context.Context, uri, dbName string) (*Mongo, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	client, err := mongo.Connect(ctx, options.Client().ApplyURI(uri))
	if err != nil {
		return nil, err
	}
	if err := client.Ping(ctx, nil); err != nil {
		_ = client.Disconnect(ctx)
		return nil, err
	}

	db := client.Database(dbName)
	m := &Mongo{
		client: client,
		tables: db.Collection("tables"),
		rounds: db.Collection("rounds"),
	}
	if err := m.ensureIndexes(ctx); err != nil {
		_ = client.Disconnect(ctx)
		return nil, err
	}
	return m, nil
}

// Close cierra la conexion con MongoDB.
func (m *Mongo) Close(ctx context.Context) error {
	return m.client.Disconnect(ctx)
}

func (m *Mongo) ensureIndexes(ctx context.Context) error {
	if _, err := m.tables.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{Keys: bson.D{{Key: "seats.userId", Value: 1}}},
		{Keys: bson.D{{Key: "status", Value: 1}}},
	}); err != nil {
		return err
	}
	_, err := m.rounds.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{Keys: bson.D{{Key: "tableId", Value: 1}, {Key: "roundNumber", Value: -1}}},
		{Keys: bson.D{{Key: "players.userId", Value: 1}}},
		{Keys: bson.D{{Key: "stage", Value: 1}}},
	})
	return err
}
