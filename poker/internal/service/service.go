package service

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"math/rand"
	"sync"
	"time"

	"github.com/ChGSufro/sis-distribuidos-casino/poker/internal/client"
	"github.com/ChGSufro/sis-distribuidos-casino/poker/internal/game"
	"github.com/ChGSufro/sis-distribuidos-casino/poker/internal/store"
	"github.com/google/uuid"
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
	ErrNotCreator      = errors.New("solo el creador puede cerrar la mesa")
	ErrTableHasPlayers = errors.New("la mesa tiene jugadores sentados")
)

// PokerService orquesta el motor de juego con la persistencia y las
// dependencias externas (Wallet, History).
type PokerService struct {
	repo    *store.Mongo
	wallet  client.WalletClient
	history client.HistoryClient
	turnTTL time.Duration
	// settleDisplay es la ventana en que la ronda settled (con winners y
	// reveals) permanece visible antes de arrancar la siguiente mano.
	settleDisplay time.Duration

	rngMu sync.Mutex
	rng   *rand.Rand

	locks *tableLocks
}

// New construye el servicio de poker.
func New(repo *store.Mongo, wallet client.WalletClient, history client.HistoryClient, turnTTL, settleDisplay time.Duration) *PokerService {
	if turnTTL <= 0 {
		turnTTL = 30 * time.Second
	}
	if settleDisplay <= 0 {
		settleDisplay = 10 * time.Second
	}
	return &PokerService{
		repo:          repo,
		wallet:        wallet,
		history:       history,
		turnTTL:       turnTTL,
		settleDisplay: settleDisplay,
		rng:           rand.New(rand.NewSource(time.Now().UnixNano())),
		locks:         newTableLocks(),
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
	tables, err := s.repo.ListTables(ctx, "")
	if err != nil {
		return
	}
	for _, t := range tables {
		if t.CurrentRoundID == "" {
			continue
		}
		s.tickTable(ctx, t.ID, now)
	}
}

// tickTable avanza la mesa en un tick: si la ronda esta settled y expiro la
// ventana de visualizacion la cierra (scrub + siguiente mano); si no,
// aplica auto-fold a los turnos expirados.
func (s *PokerService) tickTable(ctx context.Context, tableID string, now time.Time) {
	lock := s.locks.get(tableID)
	lock.Lock()
	defer lock.Unlock()

	table, err := s.repo.GetTable(ctx, tableID)
	if err != nil || table.CurrentRoundID == "" {
		return
	}
	round, err := s.repo.GetRound(ctx, table.CurrentRoundID)
	if err != nil {
		return
	}
	if round.IsSettled() {
		s.expireSettledRound(ctx, table, round, now)
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

// expireSettledRound cierra la ronda settled cuando ya paso la ventana de
// visualizacion: elimina la informacion privada de MongoDB, libera la ronda
// y arranca la siguiente mano si hay jugadores suficientes.
func (s *PokerService) expireSettledRound(ctx context.Context, table *game.Table, round *game.Round, now time.Time) {
	if round.SettledAt != nil && now.Before(round.SettledAt.Add(s.settleDisplay)) {
		return // la ronda aun se esta mostrando a los clientes
	}
	_ = s.repo.ScrubRoundCards(ctx, round.ID)
	table.CurrentRoundID = ""
	_ = s.repo.UpdateTable(ctx, table)
	_ = s.maybeStartRound(ctx, table)
}

// maybeStartRound inicia una nueva mano si hay suficientes jugadores y no hay
// una ronda en curso. Si la ronda actual esta settled pero fuera de la
// ventana de visualizacion, la cierra antes de arrancar la siguiente.
func (s *PokerService) maybeStartRound(ctx context.Context, table *game.Table) error {
	if table.Status == game.TableClosed {
		return nil
	}
	if table.CurrentRoundID != "" {
		if r, err := s.repo.GetRound(ctx, table.CurrentRoundID); err == nil {
			if !r.IsSettled() {
				return nil
			}
			if r.SettledAt != nil && time.Now().UTC().Before(r.SettledAt.Add(s.settleDisplay)) {
				return nil // la ronda settled aun se esta mostrando
			}
			_ = s.repo.ScrubRoundCards(ctx, r.ID)
			table.CurrentRoundID = ""
			if err := s.repo.UpdateTable(ctx, table); err != nil {
				return err
			}
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

// finalizeRound cierra la ronda settled: liquida los holds de los jugadores
// que se quedaron sin fichas (stack 0) y emite los eventos de historial por
// jugador (table_result). NO arranca la siguiente mano ni libera la ronda:
// los clientes con polling deben poder ver winners y reveals durante la
// ventana settleDisplay; la limpieza la hace el ticker (expireSettledRound).
func (s *PokerService) finalizeRound(ctx context.Context, table *game.Table, round *game.Round) {
	s.settleBustedSeats(ctx, table)
	s.emitTableResults(ctx, round)
}

// settleBustedSeats cierra el hold de todo asiento que se quedo sin fichas
// en esta ronda (Settle con 0: el dinero ya se perdio en la mesa), de modo
// que no queden reservas colgadas en Wallet y el asiento pueda hacer rebuy
// o abandonar sin liquidacion pendiente.
func (s *PokerService) settleBustedSeats(ctx context.Context, table *game.Table) {
	changed := false
	for i := range table.Seats {
		seat := &table.Seats[i]
		if seat.Status != game.SeatSittingOut || seat.StackCents != 0 || seat.HoldID == "" {
			continue
		}
		settleCtx, cancel := context.WithTimeout(ctx, walletTimeout)
		err := func() error {
			defer cancel()
			_, err := s.wallet.Settle(settleCtx, seat.HoldID, 0)
			return err
		}()
		if err != nil {
			log.Printf("no se pudo liquidar hold de jugador quebrado %s: %v", seat.UserID, err)
			continue // se reintenta en el proximo finalizeRound
		}
		seat.HoldID = ""
		changed = true
	}
	if changed {
		if err := s.repo.UpdateTable(ctx, table); err != nil {
			log.Printf("no se pudo persistir liquidacion de quebrados: %v", err)
		}
	}
}

// emitPlayerBuyin registra la compra de fichas de un jugador (stats suma
// totalWagered con amountCents; contrato en docs/informe-plan.md).
func (s *PokerService) emitPlayerBuyin(ctx context.Context, table *game.Table, userID string, amountCents int64, holdID string) {
	s.emitEvent(ctx, userID, table.ID, "player_buyin", map[string]interface{}{
		"tableId":     table.ID,
		"amountCents": amountCents,
		"holdId":      holdID,
	})
}

// emitPlayerPayout registra la salida de la mesa con la liquidacion final.
func (s *PokerService) emitPlayerPayout(ctx context.Context, table *game.Table, userID string, amountCents int64, holdID string) {
	s.emitEvent(ctx, userID, table.ID, "player_payout", map[string]interface{}{
		"tableId":     table.ID,
		"amountCents": amountCents,
		"holdId":      holdID,
	})
}

// cardJSON refleja el contrato de details de History (keys en minuscula).
type cardJSON struct {
	Suit string `json:"suit"`
	Rank string `json:"rank"`
}

func cardsJSON(cards []game.Card) []cardJSON {
	out := make([]cardJSON, 0, len(cards))
	for _, c := range cards {
		out = append(out, cardJSON{Suit: c.Suit, Rank: c.Rank})
	}
	return out
}

// emitTableResults registra en History un evento table_result POR JUGADOR
// de la ronda, con el contrato documentado en docs/informe-plan.md
// (stats lee yourResult y winAmountCents).
func (s *PokerService) emitTableResults(ctx context.Context, round *game.Round) {
	for i := range round.Players {
		p := &round.Players[i]
		winAmount := int64(0)
		for _, w := range round.Winners {
			if w.UserID == p.UserID {
				winAmount += w.AmountCents
			}
		}
		result := "lose"
		if winAmount > 0 {
			result = "win"
		}
		details := map[string]interface{}{
			"finalStage":     round.EndStage,
			"yourResult":     result,
			"winAmountCents": winAmount,
			"betAmountCents": p.TotalBetInRoundCents,
			"communityCards": cardsJSON(round.CommunityCards),
			"yourHand":       cardsJSON(p.HoleCards),
			"playerCount":    len(round.Players),
			"potSizeCents":   livePot(round),
		}
		s.emitEvent(ctx, p.UserID, round.ID, "table_result", details)
	}
}

// emitEvent construye y registra un evento de historial. Los fallos son
// best-effort: el cliente ya loguea [RESILIENCIA] y nunca interrumpen la
// partida.
func (s *PokerService) emitEvent(ctx context.Context, userID, gameRefID, action string, details interface{}) {
	raw, err := json.Marshal(details)
	if err != nil {
		log.Printf("[HISTORY] no se pudo serializar details de %s: %v", action, err)
		return
	}
	_ = s.history.RecordGameEvent(ctx, client.GameEvent{
		EventID:   uuid.New().String(),
		UserID:    userID,
		GameType:  "poker",
		GameRefID: gameRefID,
		Action:    action,
		Details:   string(raw),
		Timestamp: time.Now().UTC(),
	})
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
