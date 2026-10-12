package client

import (
	"context"
	"io"
	"log/slog"
	"time"

	statsPb "github.com/ChGSufro/sis-distribuidos-casino/stats/pb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// historyMethod es el método server-streaming expuesto por HistoryService.
const historyMethod = "/history.HistoryService/StreamGameEvents"

type CursorStore interface {
	LoadStreamCursor(ctx context.Context) (string, error)
	SaveStreamCursor(ctx context.Context, cursor string) error
}

type Handler interface {
	// OnEvent procesa un evento del stream. Si devuelve error, el cursor no
	// avanza y el consumidor reconecta desde el último cursor persistido.
	OnEvent(ctx context.Context, ev *statsPb.GameEvent) error
}

// HistoryStream consume el stream de History con reconexión automática.
type HistoryStream struct {
	addr        string
	minBackoff  time.Duration
	maxBackoff  time.Duration
	cursorStore CursorStore
	handler     Handler
}

func New(addr string, minBackoff, maxBackoff time.Duration, cursors CursorStore, h Handler) *HistoryStream {
	return &HistoryStream{
		addr:        addr,
		minBackoff:  minBackoff,
		maxBackoff:  maxBackoff,
		cursorStore: cursors,
		handler:     h,
	}
}

// Run ejecuta el consumidor hasta que ctx se cancele. Sin reintentos a ciegas:
// backoff exponencial 1s→30s por defecto, infinito.
func (hs *HistoryStream) Run(ctx context.Context) {
	backoff := hs.minBackoff

	for {
		err := hs.runOnce(ctx)
		switch {
		case err == nil:
			backoff = hs.minBackoff
		case ctx.Err() != nil:
			return
		default:
			slog.Error("stream de history fallo", "addr", hs.addr, "error", err)
			select {
			case <-ctx.Done():
				return
			case <-time.After(backoff):
			}
			if backoff*2 <= hs.maxBackoff {
				backoff *= 2
			}
		}
	}
}

func (hs *HistoryStream) runOnce(ctx context.Context) error {
	conn, err := grpc.NewClient(hs.addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return err
	}
	defer conn.Close()

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	cursor, err := hs.cursorStore.LoadStreamCursor(ctx)
	if err != nil {
		return err
	}

	desc := &grpc.StreamDesc{StreamName: "StreamGameEvents", ServerStreams: true}
	stream, err := conn.NewStream(ctx, desc, historyMethod)
	if err != nil {
		return err
	}

	req := &statsPb.StreamGameEventsRequest{Cursor: cursor}
	if err := stream.SendMsg(req); err != nil {
		return err
	}
	if err := stream.CloseSend(); err != nil {
		return err
	}

	slog.Info("stream de history conectado", "addr", hs.addr, "cursor", cursor)

	for {
		var ev statsPb.GameEvent
		if err := stream.RecvMsg(&ev); err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}

		if err := hs.handler.OnEvent(ctx, &ev); err != nil {
			return err
		}
		if ev.Cursor != "" {
			if err := hs.cursorStore.SaveStreamCursor(ctx, ev.Cursor); err != nil {
				return err
			}
		}
	}
}