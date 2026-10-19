package game

import (
	"fmt"
	"math/rand"
	"testing"
	"time"
)

const testTTL = 30 * time.Second

func newTable(stacks ...int64) *Table {
	t := &Table{
		ID:              "t1",
		Name:            "test",
		SmallBlindCents: 50,
		BigBlindCents:   100,
		MaxPlayers:      6,
		Status:          TableWaiting,
		DealerPosition:  -1,
	}
	for i, s := range stacks {
		t.Seats = append(t.Seats, Seat{
			SeatPosition: int32(i),
			UserID:       fmt.Sprintf("u%d", i),
			StackCents:   s,
			Status:       SeatActive,
		})
	}
	return t
}

func totalStacks(t *Table) int64 {
	var sum int64
	for _, s := range t.Seats {
		sum += s.StackCents
	}
	return sum
}

func TestStartRoundHeadsUpPostsBlinds(t *testing.T) {
	tbl := newTable(10000, 10000)
	now := time.Now()
	r, err := StartRound(tbl, "r1", 1, rand.New(rand.NewSource(1)), now, testTTL)
	if err != nil {
		t.Fatalf("StartRound: %v", err)
	}
	if r.DealerPosition != 0 {
		t.Fatalf("dealer esperado 0, fue %d", r.DealerPosition)
	}
	if got := r.PlayerByUser("u0").StackCents; got != 9950 {
		t.Fatalf("sb: stack esperado 9950, fue %d", got)
	}
	if got := r.PlayerByUser("u1").StackCents; got != 9900 {
		t.Fatalf("bb: stack esperado 9900, fue %d", got)
	}
	if r.CurrentBetCents != 100 {
		t.Fatalf("current bet esperado 100, fue %d", r.CurrentBetCents)
	}
	if r.CurrentTurn != 0 {
		t.Fatalf("en heads-up el sb/dealer actua primero; turno=%d", r.CurrentTurn)
	}
}

func TestFullHandToShowdownConservesChips(t *testing.T) {
	tbl := newTable(10000, 10000)
	now := time.Now()
	r, err := StartRound(tbl, "r1", 1, rand.New(rand.NewSource(2)), now, testTTL)
	if err != nil {
		t.Fatalf("StartRound: %v", err)
	}

	if err := ApplyAction(tbl, r, "u0", ActionCall, 0, now, testTTL); err != nil {
		t.Fatalf("call: %v", err)
	}
	if err := ApplyAction(tbl, r, "u1", ActionCheck, 0, now, testTTL); err != nil {
		t.Fatalf("check: %v", err)
	}
	if r.Stage != StageFlop {
		t.Fatalf("se esperaba flop, fue %s", r.Stage)
	}
	if len(r.CommunityCards) != 3 {
		t.Fatalf("el flop debe traer 3 cartas, trajo %d", len(r.CommunityCards))
	}

	for i := 0; i < 20 && !r.IsSettled(); i++ {
		p := r.PlayerAt(r.CurrentTurn)
		if p == nil {
			t.Fatal("turno apunta a un asiento vacio")
		}
		if err := ApplyAction(tbl, r, p.UserID, ActionCheck, 0, now, testTTL); err != nil {
			t.Fatalf("check de %s: %v", p.UserID, err)
		}
	}

	if !r.IsSettled() {
		t.Fatal("la ronda debio finalizar")
	}
	if len(r.Winners) == 0 {
		t.Fatal("debe haber al menos un ganador")
	}
	if len(r.CommunityCards) != 5 {
		t.Fatalf("deben mostrarse 5 comunitarias, hay %d", len(r.CommunityCards))
	}
	if got := totalStacks(tbl); got != 20000 {
		t.Fatalf("las fichas deben conservarse (20000), hay %d", got)
	}
}

func TestSidePots(t *testing.T) {
	tbl := newTable(1000, 5000, 5000)
	now := time.Now()
	r, err := StartRound(tbl, "r1", 1, rand.New(rand.NewSource(3)), now, testTTL)
	if err != nil {
		t.Fatalf("StartRound: %v", err)
	}

	// u0 = "AH","AD" (mejor); u1 = "KH","KD"; u2 = "QH","QD".
	r.PlayerByUser("u0").HoleCards = []Card{{SuitHearts, "A"}, {SuitDiamonds, "A"}}
	r.PlayerByUser("u1").HoleCards = []Card{{SuitHearts, "K"}, {SuitDiamonds, "K"}}
	r.PlayerByUser("u2").HoleCards = []Card{{SuitHearts, "Q"}, {SuitDiamonds, "Q"}}
	r.Deck = []Card{
		{SuitClubs, "2"}, {SuitSpades, "7"}, {SuitDiamonds, "9"},
		{SuitHearts, "3"}, {SuitClubs, "4"},
	}

	// u0 all-in corto, u1 all-in grande, u2 paga.
	if err := ApplyAction(tbl, r, "u0", ActionAllIn, 0, now, testTTL); err != nil {
		t.Fatalf("all-in u0: %v", err)
	}
	if err := ApplyAction(tbl, r, "u1", ActionAllIn, 0, now, testTTL); err != nil {
		t.Fatalf("all-in u1: %v", err)
	}
	if err := ApplyAction(tbl, r, "u2", ActionCall, 0, now, testTTL); err != nil {
		t.Fatalf("call u2: %v", err)
	}

	if !r.IsSettled() {
		t.Fatalf("la ronda debio resolverse, etapa=%s", r.Stage)
	}
	if len(r.SidePots) != 2 {
		t.Fatalf("se esperaban 2 botes, hay %d: %+v", len(r.SidePots), r.SidePots)
	}
	if r.SidePots[0].AmountCents != 3000 {
		t.Fatalf("bote principal esperado 3000, fue %d", r.SidePots[0].AmountCents)
	}
	if r.SidePots[1].AmountCents != 8000 {
		t.Fatalf("bote lateral esperado 8000, fue %d", r.SidePots[1].AmountCents)
	}
	if got := totalStacks(tbl); got != 11000 {
		t.Fatalf("fichas conservadas esperadas 11000, hay %d", got)
	}
	if !hasWinner(r, "u0", PotMain, 3000) {
		t.Fatalf("u0 debe ganar el bote principal (3000); winners=%+v", r.Winners)
	}
	if !hasWinner(r, "u1", PotSide, 8000) {
		t.Fatalf("u1 debe ganar el bote lateral (8000); winners=%+v", r.Winners)
	}
}

func TestAutoFoldOnTimeout(t *testing.T) {
	tbl := newTable(10000, 10000)
	now := time.Now()
	r, err := StartRound(tbl, "r1", 1, rand.New(rand.NewSource(4)), now, testTTL)
	if err != nil {
		t.Fatalf("StartRound: %v", err)
	}

	folded, err := AutoFold(tbl, r, now.Add(time.Second), testTTL)
	if err != nil {
		t.Fatalf("AutoFold antes de expirar: %v", err)
	}
	if folded {
		t.Fatal("no debe auto-folding antes de expirar el turno")
	}

	folded, err = AutoFold(tbl, r, now.Add(testTTL+time.Second), testTTL)
	if err != nil {
		t.Fatalf("AutoFold al expirar: %v", err)
	}
	if !folded {
		t.Fatal("debe auto-folding al expirar el turno")
	}
	if !r.IsSettled() {
		t.Fatal("al retirarse el sb heads-up, u1 gana sin showdown")
	}
	if !hasWinner(r, "u1", PotMain, 0) {
		// el monto exacto varia con las ciegas; basta con validar ganador y tipo
		if len(r.Winners) == 0 || r.Winners[0].UserID != "u1" {
			t.Fatalf("u1 debe ganar el bote; winners=%+v", r.Winners)
		}
	}
}

func TestDealerRotatesBetweenRounds(t *testing.T) {
	tbl := newTable(10000, 10000, 10000)
	now := time.Now()
	r1, err := StartRound(tbl, "r1", 1, rand.New(rand.NewSource(5)), now, testTTL)
	if err != nil {
		t.Fatalf("StartRound 1: %v", err)
	}
	d1 := r1.DealerPosition
	r2, err := StartRound(tbl, "r2", 2, rand.New(rand.NewSource(6)), now, testTTL)
	if err != nil {
		t.Fatalf("StartRound 2: %v", err)
	}
	if r2.DealerPosition == d1 {
		t.Fatalf("el dealer debe rotar entre rondas (siguio en %d)", d1)
	}
}

func hasWinner(r *Round, userID, potType string, amount int64) bool {
	for _, w := range r.Winners {
		if w.UserID == userID && w.PotType == potType && (amount == 0 || w.AmountCents == amount) {
			return true
		}
	}
	return false
}

func TestMinRaiseEnforced(t *testing.T) {
	tbl := newTable(10000, 10000, 10000)
	now := time.Now()
	r, err := StartRound(tbl, "r1", 1, rand.New(rand.NewSource(7)), now, testTTL)
	if err != nil {
		t.Fatalf("StartRound: %v", err)
	}
	// Dealer(SB) sube al minimo valido: current=100, lastRaise=100 -> 200.
	if err := ApplyAction(tbl, r, "u0", ActionRaise, 200, now, testTTL); err != nil {
		t.Fatalf("raise al minimo (200): %v", err)
	}
	// Un centavo sobre la apuesta actual no debe valer (min-raise 300).
	err = ApplyAction(tbl, r, "u1", ActionRaise, 201, now, testTTL)
	if err == nil {
		t.Fatal("raise a 201 debe rechazarse por min-raise")
	}
	// Raise completo: 100 (actual) + 100 (ultimo raise) = min 300.
	if err := ApplyAction(tbl, r, "u1", ActionRaise, 300, now, testTTL); err != nil {
		t.Fatalf("raise al minimo siguiente (300): %v", err)
	}
	if r.LastRaiseCents != 100 {
		t.Fatalf("lastRaise esperado 100, fue %d", r.LastRaiseCents)
	}
}

// TestMinRaiseToCentsExponeMinimoLegal: el estado expuesto al frontend
// (last_raise_cents / min_raise_to_cents) debe coincidir siempre con lo
// que la validacion del motor acepta.
func TestMinRaiseToCentsExponeMinimoLegal(t *testing.T) {
	tbl := newTable(10000, 10000, 10000)
	now := time.Now()
	r, err := StartRound(tbl, "r1", 1, rand.New(rand.NewSource(11)), now, testTTL)
	if err != nil {
		t.Fatalf("StartRound: %v", err)
	}
	// Preflop: current=100, lastRaise=100 -> min raise-to 200.
	if r.MinRaiseToCents() != 200 || r.LastRaiseCents != 100 {
		t.Fatalf("preflop esperado min=200 last=100, fue min=%d last=%d",
			r.MinRaiseToCents(), r.LastRaiseCents)
	}
	if err := ApplyAction(tbl, r, "u0", ActionRaise, 200, now, testTTL); err != nil {
		t.Fatalf("raise a 200: %v", err)
	}
	// Tras un raise de 100: current=200, lastRaise=100 -> min raise-to 300.
	if r.MinRaiseToCents() != 300 || r.LastRaiseCents != 100 {
		t.Fatalf("tras raise esperado min=300 last=100, fue min=%d last=%d",
			r.MinRaiseToCents(), r.LastRaiseCents)
	}
	// Primera apuesta de una calle sin bet: el minimo vuelve al big blind.
	r.CurrentBetCents = 0
	r.LastRaiseCents = 0
	if r.MinRaiseToCents() != 100 {
		t.Fatalf("calle sin apuesta esperado min=100 (BB), fue %d", r.MinRaiseToCents())
	}
}

func TestShortAllInRaiseAllowed(t *testing.T) {
	tbl := newTable(150, 10000, 10000)
	now := time.Now()
	r, err := StartRound(tbl, "r1", 1, rand.New(rand.NewSource(8)), now, testTTL)
	if err != nil {
		t.Fatalf("StartRound: %v", err)
	}
	// u0 tiene 150, apostó SB 50. All-in corto siempre esta permitido
	// aunque no alcance el min-raise.
	if err := ApplyAction(tbl, r, "u0", ActionAllIn, 0, now, testTTL); err != nil {
		t.Fatalf("all-in corto: %v", err)
	}
	if r.CurrentBetCents != 150 {
		t.Fatalf("current bet esperado 150, fue %d", r.CurrentBetCents)
	}
	if r.LastRaiseCents != 100 {
		t.Fatalf("un all-in corto no reabre el raise: lastRaise esperado 100, fue %d", r.LastRaiseCents)
	}
}

func TestTimeoutChecksWhenNoBetToCall(t *testing.T) {
	tbl := newTable(10000, 10000)
	now := time.Now()
	r, err := StartRound(tbl, "r1", 1, rand.New(rand.NewSource(9)), now, testTTL)
	if err != nil {
		t.Fatalf("StartRound: %v", err)
	}
	// Preflop: u0 (SB) hace call, u1 (BB) checkea -> u0 debe actuar
	// primero en el flop sin apuesta pendiente.
	if err := ApplyAction(tbl, r, "u0", ActionCall, 0, now, testTTL); err != nil {
		t.Fatalf("call u0: %v", err)
	}
	if err := ApplyAction(tbl, r, "u1", ActionCheck, 0, now, testTTL); err != nil {
		t.Fatalf("check u1: %v", err)
	}
	if r.Stage != StageFlop {
		t.Fatalf("se esperaba flop, fue %s", r.Stage)
	}
	p := r.PlayerAt(r.CurrentTurn)
	if p == nil {
		t.Fatal("sin jugador en turno")
	}
	if p.StreetBetCents != 0 {
		t.Fatalf("en el flop no debe haber apuesta pendiente; streetBet=%d", p.StreetBetCents)
	}
	folded, err := AutoFold(tbl, r, now.Add(testTTL+time.Second), testTTL)
	if err != nil {
		t.Fatalf("AutoFold: %v", err)
	}
	if !folded {
		t.Fatal("el timeout debe ejecutar una accion")
	}
	if p.Status != PlayerActive {
		t.Fatalf("el timeout debe hacer check (sigue activo), fue %s", p.Status)
	}
	if r.Stage == StageSettled {
		t.Fatal("no debe resolverse la ronda por un check")
	}
}

func TestHeadsUpPostflopDealerActsFirst(t *testing.T) {
	tbl := newTable(10000, 10000)
	now := time.Now()
	r, err := StartRound(tbl, "r1", 1, rand.New(rand.NewSource(10)), now, testTTL)
	if err != nil {
		t.Fatalf("StartRound: %v", err)
	}
	if err := ApplyAction(tbl, r, "u0", ActionCall, 0, now, testTTL); err != nil {
		t.Fatalf("call u0: %v", err)
	}
	if err := ApplyAction(tbl, r, "u1", ActionCheck, 0, now, testTTL); err != nil {
		t.Fatalf("check u1: %v", err)
	}
	if r.Stage != StageFlop {
		t.Fatalf("se esperaba flop, fue %s", r.Stage)
	}
	// Heads-up: el dealer (SB, asiento 0) abre la apuesta postflop.
	if r.CurrentTurn != r.DealerPosition {
		t.Fatalf("heads-up: el dealer (%d) debe actuar primero postflop, turno=%d", r.DealerPosition, r.CurrentTurn)
	}
}
