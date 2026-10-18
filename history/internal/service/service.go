package service

import (
	"context"
	"time"

	historypb "github.com/ChGSufro/sis-distribuidos-casino/history/pb"
	"github.com/ChGSufro/sis-distribuidos-casino/history/internal/broadcast"
	"github.com/ChGSufro/sis-distribuidos-casino/history/internal/store"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	DefaultLimit = 20
	MaxLimit     = 100
)

type Service struct {
	events *store.MongoStore
	broker *broadcast.Broker
}

func New(events *store.MongoStore, broker *broadcast.Broker) *Service {
	return &Service{events: events, broker: broker}
}

func (s *Service) RecordGameEvent(ctx context.Context, req *historypb.RecordGameEventRequest) (*historypb.RecordGameEventResponse, error) {
	if err := validateRecord(req); err != nil {
		return nil, err
	}

	ts, err := parseTimestamp(req.Timestamp)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "timestamp invalido: %v", err)
	}

	rec := &store.EventRecord{
		EventID:   req.EventId,
		UserID:    req.UserId,
		GameType:  req.GameType,
		GameRefID: req.GameRefId,
		Action:    req.Action,
		Details:   req.Details,
		Timestamp: ts,
	}

	exists, err := s.events.InsertEvent(ctx, rec)
	if err != nil {
		return nil, status.Errorf(codes.Unavailable, "persistir evento: %v", err)
	}

	if exists {
		// Idempotencia: el evento ya existia, se confirma sin duplicar.
		return &historypb.RecordGameEventResponse{Recorded: true, EventId: req.EventId}, nil
	}

	s.broker.Publish(toProto(rec))
	return &historypb.RecordGameEventResponse{Recorded: true, EventId: req.EventId}, nil
}

func (s *Service) GetPlayerHistory(ctx context.Context, req *historypb.GetPlayerHistoryRequest) (*historypb.GetPlayerHistoryResponse, error) {
	if req.UserId == "" {
		return nil, status.Error(codes.InvalidArgument, "user_id requerido")
	}
	limit := req.Limit
	if limit <= 0 {
		limit = DefaultLimit
	}
	if limit > MaxLimit {
		limit = MaxLimit
	}
	offset := req.Offset
	if offset < 0 {
		offset = 0
	}

	events, err := s.events.QueryHistory(ctx, req.UserId, req.GameType, int64(offset), int64(limit))
	if err != nil {
		return nil, status.Errorf(codes.Unavailable, "consultar historial: %v", err)
	}
	total, err := s.events.CountEvents(ctx, req.UserId, req.GameType)
	if err != nil {
		return nil, status.Errorf(codes.Unavailable, "contar historial: %v", err)
	}

	out := make([]*historypb.GameEvent, 0, len(events))
	for i := range events {
		out = append(out, toProto(&events[i]))
	}
	return &historypb.GetPlayerHistoryResponse{Events: out, TotalCount: int32(total)}, nil
}

// Subscribe abre un stream del broker validando el cursor.
func (s *Service) Subscribe(ctx context.Context, cursor string) (*broadcast.Subscription, error) {
	return s.broker.Subscribe(ctx, cursor, s.events)
}

func validateRecord(req *historypb.RecordGameEventRequest) error {
	if req.EventId == "" {
		return status.Error(codes.InvalidArgument, "event_id requerido")
	}
	if req.UserId == "" {
		return status.Error(codes.InvalidArgument, "user_id requerido")
	}
	if req.GameType == "" {
		return status.Error(codes.InvalidArgument, "game_type requerido")
	}
	if req.Action == "" {
		return status.Error(codes.InvalidArgument, "action requerido")
	}
	return nil
}

func parseTimestamp(v string) (time.Time, error) {
	if v == "" {
		return time.Now().UTC(), nil
	}
	ts, err := time.Parse(time.RFC3339, v)
	if err != nil {
		return time.Time{}, err
	}
	return ts.UTC(), nil
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
		Cursor:    rec.ID.Hex(),
	}
}