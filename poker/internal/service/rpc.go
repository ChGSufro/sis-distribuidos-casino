package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	pb "github.com/ChGSufro/sis-distribuidos-casino/poker/pb"
	"github.com/ChGSufro/sis-distribuidos-casino/poker/internal/game"
	"github.com/ChGSufro/sis-distribuidos-casino/poker/internal/store"
)

const walletTimeout = 5 * time.Second

func (s *PokerService) ListTables(ctx context.Context, req *pb.ListTablesRequest) (*pb.ListTablesResponse, error) {
	status := strings.TrimSpace(req.GetStatusFilter())
	tables, err := s.repo.ListTables(ctx, status)
	if err != nil {
		return nil, err
	}
	out := make([]*pb.Table, 0, len(tables))
	for _, t := range tables {
		out = append(out, tableToPB(t))
	}
	return &pb.ListTablesResponse{Tables: out}, nil
}

func (s *PokerService) CreateTable(ctx context.Context, req *pb.CreateTableRequest) (*pb.CreateTableResponse, error) {
	name := strings.TrimSpace(req.GetName())
	if name == "" || req.GetSmallBlindCents() <= 0 || req.GetBigBlindCents() <= 0 ||
		req.GetBigBlindCents() < req.GetSmallBlindCents() ||
		req.GetMaxPlayers() < 2 || req.GetMaxPlayers() > 10 {
		return nil, ErrInvalidArgument
	}

	table := &game.Table{
		Name:            name,
		SmallBlindCents: req.GetSmallBlindCents(),
		BigBlindCents:   req.GetBigBlindCents(),
		MaxPlayers:      req.GetMaxPlayers(),
		Status:          game.TableWaiting,
		DealerPosition:  -1,
	}
	if err := s.repo.CreateTable(ctx, table); err != nil {
		return nil, err
	}
	return &pb.CreateTableResponse{Table: tableToPB(table)}, nil
}

func (s *PokerService) JoinTable(ctx context.Context, req *pb.JoinTableRequest) (*pb.JoinTableResponse, error) {
	if req.GetTableId() == "" || req.GetUserId() == "" || req.GetBuyInAmountCents() <= 0 {
		return nil, ErrInvalidArgument
	}

	lock := s.locks.get(req.GetTableId())
	lock.Lock()
	defer lock.Unlock()

	table, err := s.repo.GetTable(ctx, req.GetTableId())
	if errors.Is(err, store.ErrNotFound) {
		return nil, ErrTableNotFound
	}
	if err != nil {
		return nil, err
	}
	if table.Status == game.TableClosed {
		return nil, ErrInvalidArgument
	}
	if table.SeatByUser(req.GetUserId()) != nil {
		return nil, ErrAlreadySeated
	}

	seatPos, err := assignSeat(table, req.GetPreferredSeat())
	if err != nil {
		return nil, err
	}

	holdCtx, cancel := context.WithTimeout(ctx, walletTimeout)
	defer cancel()
	holdID, err := s.wallet.Hold(holdCtx, req.GetUserId(), req.GetBuyInAmountCents(), "poker", table.ID, req.GetTableId()+":"+req.GetUserId())
	if err != nil {
		return nil, err
	}

	table.Seats = append(table.Seats, game.Seat{
		SeatPosition: seatPos,
		UserID:       req.GetUserId(),
		StackCents:   req.GetBuyInAmountCents(),
		HoldID:       holdID,
		Status:       game.SeatActive,
	})
	if err := s.repo.UpdateTable(ctx, table); err != nil {
		return nil, err
	}
	if err := s.maybeStartRound(ctx, table); err != nil {
		return nil, err
	}

	return &pb.JoinTableResponse{
		TableId:      table.ID,
		SeatPosition: seatPos,
		StackCents:   req.GetBuyInAmountCents(),
		HoldId:       holdID,
	}, nil
}

func (s *PokerService) LeaveTable(ctx context.Context, req *pb.LeaveTableRequest) (*pb.LeaveTableResponse, error) {
	if req.GetTableId() == "" || req.GetUserId() == "" {
		return nil, ErrInvalidArgument
	}

	lock := s.locks.get(req.GetTableId())
	lock.Lock()
	defer lock.Unlock()

	table, err := s.repo.GetTable(ctx, req.GetTableId())
	if errors.Is(err, store.ErrNotFound) {
		return nil, ErrTableNotFound
	}
	if err != nil {
		return nil, err
	}
	seat := table.SeatByUser(req.GetUserId())
	if seat == nil {
		return nil, ErrNotSeated
	}

	if table.CurrentRoundID != "" {
		if round, rErr := s.repo.GetRound(ctx, table.CurrentRoundID); rErr == nil && !round.IsSettled() {
			if fErr := game.ForceFold(table, round, req.GetUserId(), time.Now().UTC(), s.turnTTL); fErr == nil {
				_ = s.repo.UpdateRound(ctx, round)
				_ = s.repo.UpdateTable(ctx, table)
				if round.IsSettled() {
					s.finalizeRound(ctx, table, round)
				}
			}
		}
	}

	finalStack := seat.StackCents
	settleCtx, cancel := context.WithTimeout(ctx, walletTimeout)
	defer cancel()
	settled, err := s.wallet.Settle(settleCtx, seat.HoldID, finalStack)
	if err != nil {
		return nil, err
	}

	removeSeat(table, req.GetUserId())
	if table.Status == game.TableActive && len(table.ActiveSeats()) < 2 {
		table.Status = game.TableWaiting
	}
	if err := s.repo.UpdateTable(ctx, table); err != nil {
		return nil, err
	}

	return &pb.LeaveTableResponse{
		TableId:            table.ID,
		FinalStackCents:    finalStack,
		SettledAmountCents: settled,
	}, nil
}

func (s *PokerService) PlayerAction(ctx context.Context, req *pb.PlayerActionRequest) (*pb.PlayerActionResponse, error) {
	if req.GetTableId() == "" || req.GetUserId() == "" {
		return nil, ErrInvalidArgument
	}
	action := strings.ToLower(strings.TrimSpace(req.GetAction()))
	if !validAction(action) {
		return nil, ErrInvalidAction
	}
	if action == game.ActionRaise && req.GetAmountCents() <= 0 {
		return nil, ErrInvalidArgument
	}

	lock := s.locks.get(req.GetTableId())
	lock.Lock()
	defer lock.Unlock()

	table, err := s.repo.GetTable(ctx, req.GetTableId())
	if errors.Is(err, store.ErrNotFound) {
		return nil, ErrTableNotFound
	}
	if err != nil {
		return nil, err
	}
	round, err := s.getActiveRound(ctx, table)
	if err != nil {
		return nil, err
	}
	if req.GetRoundId() != "" && req.GetRoundId() != round.ID {
		return nil, ErrRoundNotFound
	}

	if err := game.ApplyAction(table, round, req.GetUserId(), action, req.GetAmountCents(), time.Now().UTC(), s.turnTTL); err != nil {
		return nil, mapGameError(err)
	}
	if err := s.repo.UpdateRound(ctx, round); err != nil {
		return nil, err
	}
	if err := s.repo.UpdateTable(ctx, table); err != nil {
		return nil, err
	}

	update := tableUpdate(round)
	if round.IsSettled() {
		s.finalizeRound(ctx, table, round)
	}
	return &pb.PlayerActionResponse{Update: update}, nil
}

func (s *PokerService) GetTableState(ctx context.Context, req *pb.GetTableStateRequest) (*pb.GetTableStateResponse, error) {
	if req.GetTableId() == "" {
		return nil, ErrInvalidArgument
	}
	table, err := s.repo.GetTable(ctx, req.GetTableId())
	if errors.Is(err, store.ErrNotFound) {
		return nil, ErrTableNotFound
	}
	if err != nil {
		return nil, err
	}

	resp := &pb.GetTableStateResponse{
		Table: tableToPB(table),
		Seats: seatsToPB(table),
	}
	if table.CurrentRoundID != "" {
		round, rErr := s.repo.GetRound(ctx, table.CurrentRoundID)
		if rErr == nil {
			resp.CurrentRound = roundStateFor(round, req.GetUserId())
		}
	}
	return resp, nil
}

// ───────────────────────── helpers ─────────────────────────

func assignSeat(table *game.Table, preferred int32) (int32, error) {
	if preferred >= 0 {
		if preferred >= table.MaxPlayers {
			return 0, ErrInvalidArgument
		}
		if table.SeatAt(preferred) != nil {
			return 0, ErrSeatTaken
		}
		return preferred, nil
	}
	for pos := int32(0); pos < table.MaxPlayers; pos++ {
		if table.SeatAt(pos) == nil {
			return pos, nil
		}
	}
	return 0, ErrTableFull
}

func removeSeat(table *game.Table, userID string) {
	kept := table.Seats[:0]
	for _, s := range table.Seats {
		if s.UserID != userID {
			kept = append(kept, s)
		}
	}
	table.Seats = kept
}

func validAction(action string) bool {
	switch action {
	case game.ActionFold, game.ActionCheck, game.ActionCall, game.ActionRaise, game.ActionAllIn:
		return true
	default:
		return false
	}
}

func mapGameError(err error) error {
	switch {
	case errors.Is(err, game.ErrNotYourTurn):
		return ErrNotYourTurn
	case errors.Is(err, game.ErrInvalidAction):
		return ErrInvalidAction
	case errors.Is(err, game.ErrRoundSettled):
		return ErrNoActiveRound
	default:
		return fmt.Errorf("motor: %w", err)
	}
}
