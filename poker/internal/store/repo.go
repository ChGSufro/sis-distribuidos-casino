package store

import (
	"context"
	"errors"
	"time"

	"github.com/ChGSufro/sis-distribuidos-casino/poker/internal/game"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func parseOID(id string) (primitive.ObjectID, error) {
	oid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return primitive.NilObjectID, ErrNotFound
	}
	return oid, nil
}

// ───────────────────────── tables ─────────────────────────

// CreateTable inserta una mesa y le asigna su identificador.
func (m *Mongo) CreateTable(ctx context.Context, t *game.Table) error {
	if t.CreatedAt.IsZero() {
		t.CreatedAt = time.Now().UTC()
	}
	doc := tableToDoc(t)
	doc.ID = primitive.NewObjectID()
	if _, err := m.tables.InsertOne(ctx, doc); err != nil {
		return err
	}
	t.ID = doc.ID.Hex()
	return nil
}

// GetTable obtiene una mesa por id.
func (m *Mongo) GetTable(ctx context.Context, id string) (*game.Table, error) {
	oid, err := parseOID(id)
	if err != nil {
		return nil, err
	}
	var doc tableDoc
	err = m.tables.FindOne(ctx, bson.M{"_id": oid}).Decode(&doc)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return docToTable(&doc), nil
}

// UpdateTable reemplaza la mesa completa (documento por documento).
func (m *Mongo) UpdateTable(ctx context.Context, t *game.Table) error {
	oid, err := parseOID(t.ID)
	if err != nil {
		return err
	}
	doc := tableToDoc(t)
	doc.ID = oid
	_, err = m.tables.ReplaceOne(ctx, bson.M{"_id": oid}, doc)
	return err
}

// ListTables devuelve las mesas, filtrando por estado si se indica.
func (m *Mongo) ListTables(ctx context.Context, status string) ([]*game.Table, error) {
	filter := bson.M{}
	if status != "" {
		filter["status"] = status
	}
	cur, err := m.tables.Find(ctx, filter, options.Find().SetSort(bson.D{{Key: "createdAt", Value: 1}}))
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)

	var docs []tableDoc
	if err := cur.All(ctx, &docs); err != nil {
		return nil, err
	}
	tables := make([]*game.Table, 0, len(docs))
	for i := range docs {
		tables = append(tables, docToTable(&docs[i]))
	}
	return tables, nil
}

// ───────────────────────── rounds ─────────────────────────

// CreateRound inserta una ronda y le asigna su identificador.
func (m *Mongo) CreateRound(ctx context.Context, r *game.Round) error {
	if r.CreatedAt.IsZero() {
		r.CreatedAt = time.Now().UTC()
	}
	doc := roundToDoc(r)
	doc.ID = primitive.NewObjectID()
	if _, err := m.rounds.InsertOne(ctx, doc); err != nil {
		return err
	}
	r.ID = doc.ID.Hex()
	return nil
}

// GetRound obtiene una ronda por id.
func (m *Mongo) GetRound(ctx context.Context, id string) (*game.Round, error) {
	oid, err := parseOID(id)
	if err != nil {
		return nil, err
	}
	var doc roundDoc
	err = m.rounds.FindOne(ctx, bson.M{"_id": oid}).Decode(&doc)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return docToRound(&doc), nil
}

// UpdateRound reemplaza una ronda completa.
func (m *Mongo) UpdateRound(ctx context.Context, r *game.Round) error {
	oid, err := parseOID(r.ID)
	if err != nil {
		return err
	}
	doc := roundToDoc(r)
	doc.ID = oid
	_, err = m.rounds.ReplaceOne(ctx, bson.M{"_id": oid}, doc)
	return err
}

// ScrubRoundCards elimina la informacion privada de una ronda ya cerrada
// (baraja remanente y cartas privadas) para que no quede en MongoDB.
// Se llama cuando la ronda settled sale de la ventana de visualizacion.
func (m *Mongo) ScrubRoundCards(ctx context.Context, id string) error {
	oid, err := parseOID(id)
	if err != nil {
		return err
	}
	_, err = m.rounds.UpdateByID(ctx, oid, bson.M{"$set": bson.M{
		"deck":                  []cardDoc{},
		"players.$[].holeCards": []cardDoc{},
	}})
	return err
}

// NextRoundNumber calcula el numero secuencial de la proxima mano de una mesa.
func (m *Mongo) NextRoundNumber(ctx context.Context, tableID string) (int64, error) {
	opts := options.FindOne().SetSort(bson.D{{Key: "roundNumber", Value: -1}})
	var doc roundDoc
	err := m.rounds.FindOne(ctx, bson.M{"tableId": tableID}, opts).Decode(&doc)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return 1, nil
	}
	if err != nil {
		return 0, err
	}
	return doc.RoundNumber + 1, nil
}
