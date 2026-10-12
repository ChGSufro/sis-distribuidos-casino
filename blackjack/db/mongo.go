package db

import (
	"context"
	"fmt"
	"log"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// Connect establece conexión con MongoDB reintentando ante indisponibilidad temporal y crea índices requeridos.
func Connect(ctx context.Context, uri, dbName string) (*mongo.Client, *mongo.Database, error) {
	clientOptions := options.Client().ApplyURI(uri)
	clientOptions.SetConnectTimeout(10 * time.Second)
	clientOptions.SetServerSelectionTimeout(5 * time.Second)

	var client *mongo.Client
	var err error

	maxRetries := 15
	for attempt := 1; attempt <= maxRetries; attempt++ {
		client, err = mongo.Connect(clientOptions)
		if err == nil {
			pingCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
			err = client.Ping(pingCtx, nil)
			cancel()
			if err == nil {
				log.Printf("Conexión exitosa a MongoDB (intento %d)", attempt)
				break
			}
			_ = client.Disconnect(ctx)
		}

		log.Printf("Aguardando por MongoDB (%d/%d): %v", attempt, maxRetries, err)
		select {
		case <-ctx.Done():
			return nil, nil, ctx.Err()
		case <-time.After(2 * time.Second):
		}
	}

	if err != nil {
		return nil, nil, fmt.Errorf("no se pudo conectar a MongoDB tras varios intentos: %w", err)
	}

	database := client.Database(dbName)
	if err := createIndices(ctx, database); err != nil {
		log.Printf("Advertencia al crear índices en MongoDB: %v", err)
	}

	return client, database, nil
}

func createIndices(ctx context.Context, db *mongo.Database) error {
	coll := db.Collection("hands")

	indices := []mongo.IndexModel{
		{
			Keys: bson.D{
				{Key: "userId", Value: 1},
				{Key: "createdAt", Value: -1},
			},
		},
		{
			Keys: bson.D{
				{Key: "subHands.holdId", Value: 1},
			},
		},
		{
			Keys: bson.D{
				{Key: "status", Value: 1},
			},
		},
		{
			Keys: bson.D{
				{Key: "idempotencyKey", Value: 1},
			},
			Options: options.Index().SetUnique(true),
		},
	}

	indexCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	names, err := coll.Indexes().CreateMany(indexCtx, indices)
	if err != nil {
		return fmt.Errorf("crear indices en collection hands: %w", err)
	}

	log.Printf("Índices asegurados en colección 'hands': %v", names)
	return nil
}

