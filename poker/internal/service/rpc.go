package service

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/ChGSufro/sis-distribuidos-casino/poker/internal/game"
	"github.com/ChGSufro/sis-distribuidos-casino/poker/internal/store"
	pb "github.com/ChGSufro/sis-distribuidos-casino/poker/pb"
	"github.com/google/uuid"
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
		// Sin filtro explicito el lobby no debe ver mesas cerradas.
		if status == "" && t.Status == game.TableClosed {
			continue
		}
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
	if req.GetUserId() == "" {
		return nil, ErrInvalidArgument
	}

	table := &game.Table{
		Name:            name,
		SmallBlindCents: req.GetSmallBlindCents(),
		BigBlindCents:   req.GetBigBlindCents(),
		MaxPlayers:      req.GetMaxPlayers(),
		MinBuyInCents:   req.GetMinBuyInCents(),
		MaxBuyInCents:   req.GetMaxBuyInCents(),
		Status:          game.TableWaiting,
		CreatedBy:       req.GetUserId(),
		DealerPosition:  -1,
	}
	if err := s.repo.CreateTable(ctx, table); err != nil {
		return nil, err
	}

	resp := &pb.CreateTableResponse{Table: tableToPB(table), SeatPosition: -1}
	if buyIn := req.GetBuyInAmountCents(); buyIn > 0 {
		lock := s.locks.get(table.ID)
		lock.Lock()
		defer lock.Unlock()
		// Auto-asiento del creador: es best-effort; si falla (p. ej. fondos
		// insuficientes en Wallet) se devuelve la mesa sin sentar y el
		// cliente puede unirse de forma normal.
		seatPos, holdID, err := s.seatPlayer(ctx, table, req.GetUserId(), buyIn, 0)
		if err == nil {
			if err := s.repo.UpdateTable(ctx, table); err != nil {
				// Compensacion: sin asiento persistido, liberamos la reserva.
				relCtx, cancel := context.WithTimeout(ctx, walletTimeout)
				defer cancel()
				_ = s.wallet.Release(relCtx, holdID)
				return nil, err
			}
			resp.Table = tableToPB(table)
			resp.SeatPosition = seatPos
			resp.HoldId = holdID
			s.emitPlayerBuyin(ctx, table, req.GetUserId(), buyIn, holdID)
			if err := s.maybeStartRound(ctx, table); err != nil {
				return nil, err
			}
		} else {
			log.Printf("CreateTable: auto-asiento del creador omitido: %v", err)
		}
	}
	return resp, nil
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

	seatPos, holdID, err := s.seatPlayer(ctx, table, req.GetUserId(), req.GetBuyInAmountCents(), req.GetPreferredSeat())
	if err != nil {
		return nil, err
	}
	if err := s.repo.UpdateTable(ctx, table); err != nil {
		// Compensacion: si no persistimos el asiento, liberamos la reserva.
		relCtx, cancel := context.WithTimeout(ctx, walletTimeout)
		defer cancel()
		_ = s.wallet.Release(relCtx, holdID)
		return nil, err
	}
	s.emitPlayerBuyin(ctx, table, req.GetUserId(), req.GetBuyInAmountCents(), holdID)
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

// seatPlayer sienta a un usuario en la mesa (o hace rebuy si esta sentado
// con stack 0), reservando el buy-in via Wallet.Hold. Muta table.Seats;
// el llamador es responsable de persistirla. Debe invocarse con el lock de
// la mesa adquirido salvo en CreateTable (mesa recien creada en memoria).
func (s *PokerService) seatPlayer(ctx context.Context, table *game.Table, userID string, buyIn int64, preferredSeat int32) (int32, string, error) {
	if buyIn < table.MinBuyIn() || buyIn > table.MaxBuyIn() {
		return 0, "", ErrInvalidArgument
	}

	seatPos := preferredSeat
	existing := table.SeatByUser(userID)
	if existing != nil {
		// Solo se permite re-compra: asiento con stack 0 (quebrado).
		if existing.Status != game.SeatSittingOut || existing.StackCents != 0 {
			return 0, "", ErrAlreadySeated
		}
		seatPos = existing.SeatPosition
		if existing.HoldID != "" {
			// El hold anterior cubria fichas que ya se perdieron (stack 0).
			settleCtx, cancel := context.WithTimeout(ctx, walletTimeout)
			defer cancel()
			if _, err := s.wallet.Settle(settleCtx, existing.HoldID, 0); err != nil {
				return 0, "", err
			}
			existing.HoldID = ""
		}
	} else {
		var err error
		seatPos, err = assignSeat(table, preferredSeat)
		if err != nil {
			return 0, "", err
		}
	}

	holdCtx, cancel := context.WithTimeout(ctx, walletTimeout)
	defer cancel()
	// Wallet limita idempotency_key a VARCHAR(64). El uuid garantiza unicidad
	// por intento (los rebuy requieren un hold nuevo, no deduplica).
	idemKey := fmt.Sprintf("poker:%s", uuid.New().String())
	holdID, err := s.wallet.Hold(holdCtx, userID, buyIn, "poker", table.ID, idemKey)
	if err != nil {
		return 0, "", err
	}

	if existing != nil {
		existing.StackCents = buyIn
		existing.HoldID = holdID
		existing.Status = game.SeatActive
		return seatPos, holdID, nil
	}
	table.Seats = append(table.Seats, game.Seat{
		SeatPosition: seatPos,
		UserID:       userID,
		StackCents:   buyIn,
		HoldID:       holdID,
		Status:       game.SeatActive,
	})
	return seatPos, holdID, nil
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
	settled := int64(0)
	if seat.HoldID != "" {
		// Sin hold (ya liquidado al quebrar) no hay nada que liquidar.
		settleCtx, cancel := context.WithTimeout(ctx, walletTimeout)
		defer cancel()
		var err error
		settled, err = s.wallet.Settle(settleCtx, seat.HoldID, finalStack)
		if err != nil {
			return nil, err
		}
	}

	removeSeat(table, req.GetUserId())
	if table.Status == game.TableActive && len(table.ActiveSeats()) < 2 {
		table.Status = game.TableWaiting
	}
	if err := s.repo.UpdateTable(ctx, table); err != nil {
		return nil, err
	}
	s.emitPlayerPayout(ctx, table, req.GetUserId(), finalStack, seat.HoldID)

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

// CloseTable marca la mesa como cerrada. Solo el creador puede hacerlo y
// solo si no hay jugadores sentados; las mesas cerradas desaparecen del
// lobby (ListTables las excluye por defecto) y no admiten nuevas manos.
func (s *PokerService) CloseTable(ctx context.Context, req *pb.CloseTableRequest) (*pb.CloseTableResponse, error) {
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
	if table.CreatedBy != req.GetUserId() {
		return nil, ErrNotCreator
	}
	if len(table.Seats) > 0 {
		return nil, ErrTableHasPlayers
	}

	table.Status = game.TableClosed
	if err := s.repo.UpdateTable(ctx, table); err != nil {
		return nil, err
	}
	return &pb.CloseTableResponse{TableId: table.ID, Status: game.TableClosed}, nil
}

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
