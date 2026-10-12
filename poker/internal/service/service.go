package service

import (
	"context"
	"errors"
	"math/rand"
	"sync"
	"time"

	"github.com/ChGSufro/sis-distribuidos-casino/poker/internal/client"
	"github.com/ChGSufro/sis-distribuidos-casino/poker/internal/game"
	"github.com/ChGSufro/sis-distribuidos-casino/poker/internal/store"
)

// Errores de negocio del servicio de poker.
var (
	ErrInvalidArgument = errors.New("parametros invalidos")
	ErrTableNotFound   = errors.New("mesa no encontrada")
	ErrRoundNotFound   = errors.New("ronda no encontrada")
	ErrTableFull       = errors.New("la mesa esta llena")
	ErrAlreadySeated   = errors.New("el usuario ya esta sentado en la mesa")
	ErrNotSeated       = errors.New("el usuario no esta sentado en la mesa")
	ErrNoActiveRound   = errors.New("la mesa no tiene una ronda activa")
	ErrSeatTaken       = errors.New("el asiento solicitado esta ocupado")
	ErrNotYourTurn     = errors.New("no es el turno del jugador")
	ErrInvalidAction   = errors.New("accion invalida")
)

// PokerService orquesta el motor de juego con la persistencia y las
// dependencias externas (Wallet, History).
type PokerService struct {
	repo    *store.Mongo
	wallet  client.WalletClient
	history client.HistoryClient
	turnTTL time.Duration

	rngMu sync.Mutex
	rng   *rand.Rand

	locks *tableLocks
}

// New construye el servicio de poker.
func New(repo *store.Mongo, wallet client.WalletClient, history client.HistoryClient, turnTTL time.Duration) *PokerService {
	if turnTTL <= 0 {
		turnTTL = 30 * time.Second
	}
	return &PokerService{
		repo:    repo,
		wallet:  wallet,
		history: history,
		turnTTL: turnTTL,
		rng:     rand.New(rand.NewSource(time.Now().UnixNano())),
		locks:   newTableLocks(),
	}
}

func (s *PokerService) nextRNG() *rand.Rand {
	s.rngMu.Lock()
	defer s.rngMu.Unlock()
	return s.rng
}

// RunAutoFold ejecuta periodicamente el auto-fold de los turnos expirados.
func (s *PokerService) RunAutoFold(ctx context.Context, interval time.Duration) {
	if interval <= 0 {
		interval = time.Second
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case now := <-ticker.C:
			s.autoFoldOnce(ctx, now)
		}
	}
}

func (s *PokerService) autoFoldOnce(ctx context.Context, now time.Time) {
	tables, err := s.repo.ListTables(ctx, game.TableActive)
	if err != nil {
		return
	}
	for _, t := range tables {
		if t.CurrentRoundID == "" {
			continue
		}
		s.autoFoldTable(ctx, t.ID, now)
	}
}

func (s *PokerService) autoFoldTable(ctx context.Context, tableID string, now time.Time) {
	lock := s.locks.get(tableID)
	lock.Lock()
	defer lock.Unlock()

	table, err := s.repo.GetTable(ctx, tableID)
	if err != nil || table.CurrentRoundID == "" {
		return
	}
	round, err := s.repo.GetRound(ctx, table.CurrentRoundID)
	if err != nil || round.IsSettled() {
		return
	}
	changed, err := game.AutoFold(table, round, now, s.turnTTL)
	if err != nil || !changed {
		return
	}
	_ = s.repo.UpdateRound(ctx, round)
	_ = s.repo.UpdateTable(ctx, table)
	if round.IsSettled() {
		s.finalizeRound(ctx, table, round)
	}
}

// maybeStartRound inicia una nueva mano si hay suficientes jugadores y no hay
// una ronda en curso.
func (s *PokerService) maybeStartRound(ctx context.Context, table *game.Table) error {
	if table.Status == game.TableClosed {
		return nil
	}
	if table.CurrentRoundID != "" {
		if r, err := s.repo.GetRound(ctx, table.CurrentRoundID); err == nil && !r.IsSettled() {
			return nil
		}
	}
	if len(table.ActiveSeats()) < 2 {
		return nil
	}

	roundNumber, err := s.repo.NextRoundNumber(ctx, table.ID)
	if err != nil {
		return err
	}
	round, err := game.StartRound(table, "", roundNumber, s.nextRNG(), time.Now().UTC(), s.turnTTL)
	if err != nil {
		return err
	}
	if err := s.repo.CreateRound(ctx, round); err != nil {
		return err
	}
	table.CurrentRoundID = round.ID
	return s.repo.UpdateTable(ctx, table)
}

// finalizeRound cierra la ronda: la audita en History y, si aun hay jugadores,
// comienza la siguiente mano.
func (s *PokerService) finalizeRound(ctx context.Context, table *game.Table, round *game.Round) {
	event := client.GameEvent{
		GameType:   "poker",
		TableID:    table.ID,
		RoundID:    round.ID,
		PotCents:   round.PotSizeCents,
		FinishedAt: time.Now().UTC(),
	}
	for _, w := range round.Winners {
		event.Winners = append(event.Winners, client.WinnerInfo{UserID: w.UserID, AmountCents: w.AmountCents})
	}
	_ = s.history.RecordGameEvent(ctx, event)

	table.CurrentRoundID = ""
	_ = s.repo.UpdateTable(ctx, table)
	_ = s.maybeStartRound(ctx, table)
}

func (s *PokerService) getActiveRound(ctx context.Context, table *game.Table) (*game.Round, error) {
	if table.CurrentRoundID == "" {
		return nil, ErrNoActiveRound
	}
	round, err := s.repo.GetRound(ctx, table.CurrentRoundID)
	if errors.Is(err, store.ErrNotFound) {
		return nil, ErrRoundNotFound
	}
	if err != nil {
		return nil, err
	}
	return round, nil
}

// tableLocks serializa las operaciones por mesa.
type tableLocks struct {
	mu    sync.Mutex
	locks map[string]*sync.Mutex
}

func newTableLocks() *tableLocks {
	return &tableLocks{locks: make(map[string]*sync.Mutex)}
}

func (l *tableLocks) get(id string) *sync.Mutex {
	l.mu.Lock()
	defer l.mu.Unlock()
	m, ok := l.locks[id]
	if !ok {
		m = &sync.Mutex{}
		l.locks[id] = m
	}
	return m
}
