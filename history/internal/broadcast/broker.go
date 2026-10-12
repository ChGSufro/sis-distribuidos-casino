package broadcast

import (
	"context"
	"sync"
	"time"

	historypb "github.com/ChGSufro/sis-distribuidos-casino/history/pb"
	"github.com/ChGSufro/sis-distribuidos-casino/history/internal/store"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

const (
	queueCapacity = 1024
	catchUpBatch  = 1000
	catchUpRetry  = 500 * time.Millisecond
)

// EventSource provee los eventos históricos para el catch-up de un stream.
type EventSource interface {
	EventsAfterCursor(ctx context.Context, cursor string, limit int64) ([]store.EventRecord, error)
}

// Subscription entrega eventos en orden FIFO sin pérdidas. Primero recibe los
// eventos históricos posteriores al cursor y luego los eventos en vivo.
type Subscription struct {
	broker *Broker
	id     uint64

	mu     sync.Mutex
	queue  []*historypb.GameEvent
	notify chan struct{}
	done   chan struct{}
	out    chan *historypb.GameEvent
	closed bool
}

func (s *Subscription) enqueue(ev *historypb.GameEvent) {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return
	}
	s.queue = append(s.queue, ev)
	s.mu.Unlock()

	select {
	case s.notify <- struct{}{}:
	default:
	}
}

func (s *Subscription) run() {
	for {
		select {
		case <-s.done:
			return
		case <-s.notify:
		}
		s.mu.Lock()
		batch := s.queue
		s.queue = nil
		s.mu.Unlock()

		for _, ev := range batch {
			select {
			case s.out <- ev:
			case <-s.done:
				return
			}
		}
	}
}

// Events devuelve el canal FIFO de eventos del stream.
func (s *Subscription) Events() <-chan *historypb.GameEvent {
	return s.out
}

// Close cierra la suscripcion y la desregistra del broker.
func (s *Subscription) Close() {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return
	}
	s.closed = true
	s.mu.Unlock()

	s.broker.unregister(s)
	close(s.done)
}

func (s *Subscription) catchUp(ctx context.Context, cursor string, src EventSource) {
	last := cursor
	for {
		recs, err := src.EventsAfterCursor(ctx, last, catchUpBatch)
		if err != nil {
			select {
			case <-ctx.Done():
				return
			case <-time.After(catchUpRetry):
				continue
			}
		}
		if len(recs) == 0 {
			return
		}
		for i := range recs {
			last = recs[i].ID.Hex()
			s.enqueue(toProto(&recs[i]))
		}
		if int64(len(recs)) < catchUpBatch {
			return
		}
	}
}

func toProto(rec *store.EventRecord) *historypb.GameEvent {
	return &historypb.GameEvent{
		EventId:   rec.EventID,
		UserId:    rec.UserID,
		GameType:  rec.GameType,
		GameRefId: rec.GameRefID,
		Action:    rec.Action,
		Details:   rec.Details,
		Timestamp: rec.Timestamp.UTC().Format(time.RFC3339),
	}
}

type Broker struct {
	mu     sync.Mutex
	subs   map[uint64]*Subscription
	nextID uint64
}

func NewBroker() *Broker {
	return &Broker{subs: make(map[uint64]*Subscription)}
}

// Subscribe registra un suscriptor. cursor es un ObjectId hex del ultimo evento
// recibido por el consumidor; vacio indica comenzar desde el inicio.
func (b *Broker) Subscribe(ctx context.Context, cursor string, src EventSource) (*Subscription, error) {
	if cursor != "" {
		if _, err := primitive.ObjectIDFromHex(cursor); err != nil {
			return nil, status.Error(codes.InvalidArgument, "cursor invalido: debe ser un ObjectId hex")
		}
	}

	b.mu.Lock()
	b.nextID++
	sub := &Subscription{
		broker: b,
		id:     b.nextID,
		notify: make(chan struct{}, 1),
		done:   make(chan struct{}),
		out:    make(chan *historypb.GameEvent, queueCapacity),
	}
	b.subs[sub.id] = sub
	b.mu.Unlock()

	go sub.run()
	go sub.catchUp(ctx, cursor, src)
	return sub, nil
}

func (b *Broker) unregister(sub *Subscription) {
	b.mu.Lock()
	delete(b.subs, sub.id)
	b.mu.Unlock()
}

// Publish difunde un evento en vivo a todos los suscriptores activos.
func (b *Broker) Publish(ev *historypb.GameEvent) {
	b.mu.Lock()
	defer b.mu.Unlock()
	for _, sub := range b.subs {
		sub.enqueue(ev)
	}
}