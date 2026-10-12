package handler

import (
	"context"

	"github.com/ChGSufro/sis-distribuidos-casino/history/internal/service"
	historypb "github.com/ChGSufro/sis-distribuidos-casino/history/pb"
	"google.golang.org/grpc"
)

type Handler struct {
	historypb.UnimplementedHistoryServiceServer
	svc *service.Service
}

func New(svc *service.Service) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) RecordGameEvent(ctx context.Context, req *historypb.RecordGameEventRequest) (*historypb.RecordGameEventResponse, error) {
	return h.svc.RecordGameEvent(ctx, req)
}

func (h *Handler) GetPlayerHistory(ctx context.Context, req *historypb.GetPlayerHistoryRequest) (*historypb.GetPlayerHistoryResponse, error) {
	return h.svc.GetPlayerHistory(ctx, req)
}

func (h *Handler) StreamGameEvents(req *historypb.StreamGameEventsRequest, stream grpc.ServerStreamingServer[historypb.GameEvent]) error {
	sub, err := h.svc.Subscribe(stream.Context(), req.Cursor)
	if err != nil {
		return err
	}
	defer sub.Close()

	for {
		select {
		case ev := <-sub.Events():
			if err := stream.Send(ev); err != nil {
				return err
			}
		case <-stream.Context().Done():
			return nil
		}
	}
}