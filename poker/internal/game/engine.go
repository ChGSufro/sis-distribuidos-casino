package game

import (
	"errors"
	"math/rand"
	"sort"
	"time"
)

// Errores del motor.
var (
	ErrNotEnoughPlayers = errors.New("se requieren al menos 2 jugadores activos")
	ErrNotYourTurn      = errors.New("no es el turno del jugador")
	ErrInvalidAction    = errors.New("accion invalida")
	ErrRoundSettled     = errors.New("la ronda ya finalizo")
)

const holeCards = 2

// StartRound crea una nueva mano sobre la mesa: rota el boton, cobra las
// ciegas, reparte las cartas y deja la accion en preflop. Muta la mesa
// (dealer, currentRoundId y stacks) y devuelve la ronda.
func StartRound(table *Table, roundID string, roundNumber int64, rng *rand.Rand, now time.Time, ttl time.Duration) (*Round, error) {
	seats := table.ActiveSeats()
	if len(seats) < 2 {
		return nil, ErrNotEnoughPlayers
	}
	sort.Slice(seats, func(i, j int) bool { return seats[i].SeatPosition < seats[j].SeatPosition })

	dealer := rotateDealer(table, seats)
	table.DealerPosition = dealer.SeatPosition

	n := len(seats)
	dealerIdx := indexOfSeat(seats, dealer.SeatPosition)
	var sb, bb Seat
	if n == 2 {
		sb = seats[dealerIdx]
		bb = seats[(dealerIdx+1)%n]
	} else {
		sb = seats[(dealerIdx+1)%n]
		bb = seats[(dealerIdx+2)%n]
	}

	deck := NewDeck()
	Shuffle(deck, rng)

	round := &Round{
		ID:              roundID,
		TableID:         table.ID,
		RoundNumber:     roundNumber,
		DealerPosition:  dealer.SeatPosition,
		CurrentBetCents: table.BigBlindCents,
		Stage:           StagePreflop,
		BigBlindCents:   table.BigBlindCents,
		LastRaiseCents:  table.BigBlindCents,
		CreatedAt:       now,
		Deck:            deck,
	}
	for _, s := range seats {
		round.Players = append(round.Players, RoundPlayerFromSeat(s))
	}

	for i := range round.Players {
		var cards []Card
		cards, round.Deck = Deal(round.Deck, holeCards)
		round.Players[i].HoleCards = cards
	}

	postBlind(round, sb.SeatPosition, table.SmallBlindCents)
	postBlind(round, bb.SeatPosition, table.BigBlindCents)

	if table.Status == TableWaiting {
		table.Status = TableActive
	}
	table.CurrentRoundID = round.ID

	turn := nextActiveAfter(round, bb.SeatPosition)
	if turn == nil {
		syncTableStacks(table, round)
		return round, advanceStreet(table, round, now, ttl)
	}
	round.CurrentTurn = turn.SeatPosition
	round.TurnExpiresAt = now.Add(ttl)
	syncTableStacks(table, round)
	return round, nil
}

// ApplyAction registra la accion de un jugador y avanza la ronda.
func ApplyAction(table *Table, round *Round, userID, action string, amount int64, now time.Time, ttl time.Duration) error {
	if round.IsSettled() {
		return ErrRoundSettled
	}
	p := round.PlayerByUser(userID)
	if p == nil {
		return ErrInvalidAction
	}
	if p.SeatPosition != round.CurrentTurn {
		return ErrNotYourTurn
	}
	return applyPlayerAction(table, round, p, action, amount, now, ttl)
}

// ForceFold retira a un jugador de la ronda al margen de su turno. Se usa,
// por ejemplo, cuando un jugador abandona la mesa a mitad de mano.
func ForceFold(table *Table, round *Round, userID string, now time.Time, ttl time.Duration) error {
	if round.IsSettled() {
		return ErrRoundSettled
	}
	p := round.PlayerByUser(userID)
	if p == nil || p.Status != PlayerActive {
		return nil
	}
	p.Status = PlayerFolded
	p.HasActed = true
	round.Actions = append(round.Actions, Action{
		UserID:    p.UserID,
		Action:    ActionFold,
		Timestamp: now,
	})
	return advanceIfReady(table, round, now, ttl)
}

// AutoFold aplica la accion por defecto al jugador en turno si expiro su
// tiempo. Si no hay apuesta que igualar comprueba (check); en caso
// contrario se retira (fold). Devuelve true si efectivamente actuó.
func AutoFold(table *Table, round *Round, now time.Time, ttl time.Duration) (bool, error) {
	if round.IsSettled() || round.Stage == StageShowdown {
		return false, nil
	}
	if now.Before(round.TurnExpiresAt) {
		return false, nil
	}
	p := round.PlayerAt(round.CurrentTurn)
	if p == nil || p.Status != PlayerActive {
		return false, nil
	}
	// Sin apuesta que igualar, el timeout comprueba en lugar de retirarse
	// (un jugador desconectado no deberia perder la mano por nada).
	action := ActionFold
	if p.StreetBetCents == round.CurrentBetCents {
		action = ActionCheck
	}
	if err := applyPlayerAction(table, round, p, action, 0, now, ttl); err != nil {
		return false, err
	}
	return true, nil
}

func minRaiseTarget(round *Round) int64 {
	return round.MinRaiseToCents()
}

func applyPlayerAction(table *Table, round *Round, p *RoundPlayer, action string, amount int64, now time.Time, ttl time.Duration) error {
	switch action {
	case ActionFold:
		p.Status = PlayerFolded
	case ActionCheck:
		if p.StreetBetCents != round.CurrentBetCents {
			return ErrInvalidAction
		}
	case ActionCall:
		commit(p, round.CurrentBetCents-p.StreetBetCents)
	case ActionRaise:
		if amount <= round.CurrentBetCents {
			return ErrInvalidAction
		}
		// Min-raise: el raise debe alcanzar la apuesta actual mas el
		// tamano del ultimo raise completo. La unica excepcion es ir
		// all-in (raise corto), siempre permitido.
		chips := amount - p.StreetBetCents
		if chips > p.StackCents {
			chips = p.StackCents // se clampkea: el jugador queda all-in
		}
		if amount < minRaiseTarget(round) && p.StackCents > chips {
			return ErrInvalidAction
		}
		commit(p, chips)
	case ActionAllIn:
		commit(p, p.StackCents)
	default:
		return ErrInvalidAction
	}

	p.HasActed = true
	round.Actions = append(round.Actions, Action{
		UserID:      p.UserID,
		Action:      action,
		AmountCents: actionAmount(action, p, amount),
		Timestamp:   now,
	})

	if p.Status == PlayerActive && p.StreetBetCents > round.CurrentBetCents {
		noteRaise(round, action, p.StreetBetCents-round.CurrentBetCents)
		round.CurrentBetCents = p.StreetBetCents
		resetActed(round, p)
	}
	if p.Status == PlayerAllIn && p.StreetBetCents > round.CurrentBetCents {
		noteRaise(round, action, p.StreetBetCents-round.CurrentBetCents)
		round.CurrentBetCents = p.StreetBetCents
		resetActed(round, p)
	}

	return advanceIfReady(table, round, now, ttl)
}

// advanceIfReady decide si la ronda debe terminar, cambiar de calle o pasar
// el turno al siguiente jugador.
func advanceIfReady(table *Table, round *Round, now time.Time, ttl time.Duration) error {
	if len(round.Contenders()) <= 1 {
		return settleUncontested(table, round, now)
	}
	if bettingComplete(round) {
		return advanceStreet(table, round, now, ttl)
	}
	next := nextActiveAfter(round, round.CurrentTurn)
	if next == nil {
		return settleShowdown(table, round, now)
	}
	round.CurrentTurn = next.SeatPosition
	round.TurnExpiresAt = now.Add(ttl)
	return nil
}

func bettingComplete(round *Round) bool {
	for i := range round.Players {
		p := &round.Players[i]
		if p.Status != PlayerActive {
			continue
		}
		if !p.HasActed || p.StreetBetCents != round.CurrentBetCents {
			return false
		}
	}
	return true
}

// advanceStreet pasa a la siguiente calle, reparte cartas comunitarias y
// resetea las apuestas. Si ya no es posible apostar, corre las calles
// restantes hasta el showdown.
func advanceStreet(table *Table, round *Round, now time.Time, ttl time.Duration) error {
	for {
		switch round.Stage {
		case StagePreflop:
			round.Stage = StageFlop
			dealCommunity(round, 3)
		case StageFlop:
			round.Stage = StageTurn
			dealCommunity(round, 1)
		case StageTurn:
			round.Stage = StageRiver
			dealCommunity(round, 1)
		case StageRiver:
			return settleShowdown(table, round, now)
		default:
			return settleShowdown(table, round, now)
		}

		resetStreet(round)
		if countCanAct(round) <= 1 {
			continue
		}
		next := firstToActPostflop(round)
		if next == nil {
			continue
		}
		round.CurrentTurn = next.SeatPosition
		round.TurnExpiresAt = now.Add(ttl)
		return nil
	}
}

// firstToActPostflop devuelve quien abre la apuesta en la calle nueva.
// En mesa llena es el SB (siguiente tras el dealer); en heads-up el
// dealer/SB es quien actua primero tambien postflop.
func firstToActPostflop(round *Round) *RoundPlayer {
	if len(round.Players) == 2 {
		if dealer := round.PlayerAt(round.DealerPosition); dealer != nil && dealer.Status == PlayerActive {
			return dealer
		}
	}
	return nextActiveAfter(round, round.DealerPosition)
}

// settleUncontested resuelve la ronda cuando solo queda un jugador sin
// retirarse: se lleva todo el bote sin showdown.
func settleUncontested(table *Table, round *Round, now time.Time) error {
	var winner *RoundPlayer
	for i := range round.Players {
		if round.Players[i].Status != PlayerFolded {
			winner = &round.Players[i]
			break
		}
	}
	if winner == nil {
		return finishRound(table, round, now)
	}
	total := int64(0)
	for i := range round.Players {
		total += round.Players[i].TotalBetInRoundCents
	}
	winner.StackCents += total
	round.PotSizeCents = total
	round.Winners = []Winner{{UserID: winner.UserID, PotType: PotMain, AmountCents: total}}
	return finishRound(table, round, now)
}

// settleShowdown evalua las manos, reparte los botes (principal y laterales)
// y registra a los ganadores.
func settleShowdown(table *Table, round *Round, now time.Time) error {
	round.Stage = StageShowdown
	pots := buildPots(round)

	for idx, pot := range pots {
		potType := PotMain
		if idx > 0 {
			potType = PotSide
		}
		winners := bestContenders(round, pot.EligiblePlayers)
		if len(winners) == 0 {
			continue
		}
		sort.Slice(winners, func(i, j int) bool { return winners[i].SeatPosition < winners[j].SeatPosition })
		share := pot.AmountCents / int64(len(winners))
		remainder := pot.AmountCents % int64(len(winners))
		for i, w := range winners {
			amt := share
			if int64(i) < remainder {
				amt++
			}
			w.StackCents += amt
			round.Winners = append(round.Winners, Winner{UserID: w.UserID, PotType: potType, AmountCents: amt})
		}
	}
	round.SidePots = pots
	return finishRound(table, round, now)
}

func finishRound(table *Table, round *Round, now time.Time) error {
	round.EndStage = round.Stage // guarda la etapa real de cierre antes de marcar settled
	round.Stage = StageSettled
	t := now
	round.SettledAt = &t
	total := int64(0)
	for i := range round.Players {
		total += round.Players[i].TotalBetInRoundCents
	}
	round.PotSizeCents = total
	syncTableStacks(table, round)
	return nil
}

// buildPots construye el bote principal y los laterales a partir de las
// contribuciones de cada jugador. Los jugadores retirados aportan al bote
// pero no pueden ganarlo.
func buildPots(round *Round) []SidePot {
	type entry struct {
		user   string
		amt    int64
		folded bool
	}
	entries := make([]entry, 0, len(round.Players))
	for i := range round.Players {
		p := &round.Players[i]
		if p.TotalBetInRoundCents <= 0 {
			continue
		}
		entries = append(entries, entry{user: p.UserID, amt: p.TotalBetInRoundCents, folded: p.Status == PlayerFolded})
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].amt < entries[j].amt })

	pots := make([]SidePot, 0)
	prev := int64(0)
	for _, e := range entries {
		if e.amt <= prev {
			continue
		}
		slice := e.amt - prev
		count := int64(0)
		eligible := make([]string, 0)
		for _, o := range entries {
			if o.amt >= e.amt {
				count++
				if !o.folded {
					eligible = append(eligible, o.user)
				}
			}
		}
		if amount := slice * count; amount > 0 {
			pots = append(pots, SidePot{AmountCents: amount, EligiblePlayers: eligible})
		}
		prev = e.amt
	}
	return pots
}

// bestContenders devuelve los jugadores elegibles con la mejor mano.
func bestContenders(round *Round, eligible []string) []*RoundPlayer {
	var winners []*RoundPlayer
	var best HandRank
	first := true
	for _, userID := range eligible {
		p := round.PlayerByUser(userID)
		if p == nil || p.Status == PlayerFolded {
			continue
		}
		cards := make([]Card, 0, len(p.HoleCards)+len(round.CommunityCards))
		cards = append(cards, p.HoleCards...)
		cards = append(cards, round.CommunityCards...)
		rank := BestHand(cards)
		if first {
			best, winners, first = rank, []*RoundPlayer{p}, false
			continue
		}
		switch c := rank.Compare(best); {
		case c > 0:
			best, winners = rank, []*RoundPlayer{p}
		case c == 0:
			winners = append(winners, p)
		}
	}
	return winners
}

// ───────────────────────── helpers ─────────────────────────

func commit(p *RoundPlayer, amount int64) {
	if amount < 0 {
		amount = 0
	}
	if amount > p.StackCents {
		amount = p.StackCents
	}
	p.StackCents -= amount
	p.StreetBetCents += amount
	p.TotalBetInRoundCents += amount
	if p.StackCents == 0 {
		p.Status = PlayerAllIn
	}
}

func postBlind(round *Round, pos int32, amount int64) {
	p := round.PlayerAt(pos)
	if p == nil {
		return
	}
	commit(p, amount)
	p.HasActed = false
}

func resetActed(round *Round, aggressor *RoundPlayer) {
	for i := range round.Players {
		p := &round.Players[i]
		if p == aggressor || p.Status != PlayerActive {
			continue
		}
		p.HasActed = false
	}
}

// noteRaise actualiza el tamano del ultimo raise completo. Solo un raise
// (o all-in) igual o mayor que el anterior cuenta como tal: un all-in
// corto no reabre el min-raise.
func noteRaise(round *Round, action string, delta int64) {
	if action != ActionRaise && action != ActionAllIn {
		return
	}
	if delta >= round.LastRaiseCents {
		round.LastRaiseCents = delta
	}
}

func resetStreet(round *Round) {
	round.CurrentBetCents = 0
	round.LastRaiseCents = 0
	for i := range round.Players {
		p := &round.Players[i]
		p.StreetBetCents = 0
		if p.Status == PlayerActive {
			p.HasActed = false
		}
	}
}

func dealCommunity(round *Round, n int) {
	var cards []Card
	cards, round.Deck = Deal(round.Deck, n)
	round.CommunityCards = append(round.CommunityCards, cards...)
}

func actionAmount(action string, p *RoundPlayer, requested int64) int64 {
	switch action {
	case ActionFold, ActionCheck:
		return 0
	case ActionRaise:
		return requested
	default:
		return p.TotalBetInRoundCents
	}
}

func countCanAct(round *Round) int {
	n := 0
	for i := range round.Players {
		if round.Players[i].Status == PlayerActive {
			n++
		}
	}
	return n
}

// nextActiveAfter devuelve el siguiente jugador que puede actuar tras pos,
// en orden circular de asientos. Con pos = -1 devuelve el primero.
func nextActiveAfter(round *Round, pos int32) *RoundPlayer {
	var wrap *RoundPlayer
	for i := range round.Players {
		p := &round.Players[i]
		if p.Status != PlayerActive {
			continue
		}
		if wrap == nil {
			wrap = p
		}
		if p.SeatPosition > pos {
			return p
		}
	}
	return wrap
}

func rotateDealer(table *Table, seats []Seat) Seat {
	from := table.DealerPosition
	for _, s := range seats {
		if s.SeatPosition > from {
			return s
		}
	}
	return seats[0]
}

func indexOfSeat(seats []Seat, pos int32) int {
	for i, s := range seats {
		if s.SeatPosition == pos {
			return i
		}
	}
	return 0
}

func syncTableStacks(table *Table, round *Round) {
	for i := range table.Seats {
		p := round.PlayerAt(table.Seats[i].SeatPosition)
		if p == nil {
			continue
		}
		table.Seats[i].StackCents = p.StackCents
		if table.Seats[i].Status == SeatActive && p.StackCents == 0 {
			table.Seats[i].Status = SeatSittingOut
		}
	}
}
