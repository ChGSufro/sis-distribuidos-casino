// Package game implementa el dominio y el motor de Texas Hold'em No-Limit.
// Es independiente del transporte (gRPC) y de la persistencia (MongoDB),
// de modo que pueda probarse de forma pura.
package game

import "time"

// Palos de la baraja.
const (
	SuitHearts   = "H"
	SuitDiamonds = "D"
	SuitClubs    = "C"
	SuitSpades   = "S"
)

// Estados de la mesa.
const (
	TableWaiting = "waiting"
	TableActive  = "active"
	TableClosed  = "closed"
)

// Estados de asiento.
const (
	SeatActive     = "active"
	SeatSittingOut = "sitting_out"
	SeatLeft       = "left"
)

// Estados de jugador dentro de la ronda.
const (
	PlayerActive = "active"
	PlayerFolded = "folded"
	PlayerAllIn  = "all-in"
)

// Etapas de la ronda.
const (
	StagePreflop  = "preflop"
	StageFlop     = "flop"
	StageTurn     = "turn"
	StageRiver    = "river"
	StageShowdown = "showdown"
	StageSettled  = "settled"
)

// Acciones validas.
const (
	ActionFold  = "fold"
	ActionCheck = "check"
	ActionCall  = "call"
	ActionRaise = "raise"
	ActionAllIn = "all-in"
)

// Tipos de bote para los ganadores.
const (
	PotMain = "main"
	PotSide = "side"
)

// Card es una carta de la baraja.
type Card struct {
	Suit string
	Rank string
}

// Seat es un jugador sentado en una mesa persistente.
type Seat struct {
	SeatPosition int32
	UserID       string
	StackCents   int64
	HoldID       string
	Status       string
}

// Table es la entidad persistente de una mesa; sobrevive entre rondas.
type Table struct {
	ID              string
	Name            string
	SmallBlindCents int64
	BigBlindCents   int64
	MaxPlayers      int32
	Status          string
	Seats           []Seat
	CurrentRoundID  string
	DealerPosition  int32
	CreatedAt       time.Time
}

// ActiveSeats devuelve los asientos que participan de una ronda. Un asiento
// con stack 0 queda fuera (eliminado).
func (t *Table) ActiveSeats() []Seat {
	out := make([]Seat, 0, len(t.Seats))
	for _, s := range t.Seats {
		if s.Status == SeatActive && s.StackCents > 0 {
			out = append(out, s)
		}
	}
	return out
}

// SeatByUser devuelve el asiento de un usuario, o nil si no esta sentado.
func (t *Table) SeatByUser(userID string) *Seat {
	for i := range t.Seats {
		if t.Seats[i].UserID == userID {
			return &t.Seats[i]
		}
	}
	return nil
}

// SeatAt devuelve el asiento en una posicion, o nil si esta libre.
func (t *Table) SeatAt(pos int32) *Seat {
	for i := range t.Seats {
		if t.Seats[i].SeatPosition == pos {
			return &t.Seats[i]
		}
	}
	return nil
}

// Action es una accion registrada en la ronda.
type Action struct {
	UserID      string
	Action      string
	AmountCents int64
	Timestamp   time.Time
}

// SidePot es un bote secundario originado por all-in.
type SidePot struct {
	AmountCents     int64
	EligiblePlayers []string
}

// Winner registra el resultado de un bote al cerrar la ronda.
type Winner struct {
	UserID      string
	PotType     string
	AmountCents int64
}

// RoundPlayer es el estado de un jugador dentro de una ronda.
type RoundPlayer struct {
	UserID               string
	SeatPosition         int32
	HoleCards            []Card
	StackAtStartCents    int64
	StackCents           int64
	TotalBetInRoundCents int64
	StreetBetCents       int64
	HasActed             bool
	Status               string
}

// FromSeat construye el estado de ronda a partir de un asiento.
func RoundPlayerFromSeat(s Seat) RoundPlayer {
	return RoundPlayer{
		UserID:            s.UserID,
		SeatPosition:      s.SeatPosition,
		StackAtStartCents: s.StackCents,
		StackCents:        s.StackCents,
		Status:            PlayerActive,
	}
}

// Round es una mano individual dentro de una mesa.
type Round struct {
	ID             string
	TableID        string
	RoundNumber    int64
	Players        []RoundPlayer
	CommunityCards []Card
	PotSizeCents   int64
	SidePots       []SidePot
	DealerPosition int32
	CurrentTurn    int32
	CurrentBetCents int64
	TurnExpiresAt  time.Time
	Stage          string
	Actions        []Action
	Winners        []Winner
	Deck           []Card
	CreatedAt      time.Time
	SettledAt      *time.Time
}

// PlayerByUser devuelve el jugador de la ronda por userId, o nil.
func (r *Round) PlayerByUser(userID string) *RoundPlayer {
	for i := range r.Players {
		if r.Players[i].UserID == userID {
			return &r.Players[i]
		}
	}
	return nil
}

// PlayerAt devuelve el jugador de la ronda por posicion de asiento, o nil.
func (r *Round) PlayerAt(pos int32) *RoundPlayer {
	for i := range r.Players {
		if r.Players[i].SeatPosition == pos {
			return &r.Players[i]
		}
	}
	return nil
}

// IsSettled indica si la ronda ya fue resuelta.
func (r *Round) IsSettled() bool {
	return r.Stage == StageSettled
}

// Contenders devuelve los jugadores que aun no se han retirado.
func (r *Round) Contenders() []*RoundPlayer {
	out := make([]*RoundPlayer, 0, len(r.Players))
	for i := range r.Players {
		if r.Players[i].Status != PlayerFolded {
			out = append(out, &r.Players[i])
		}
	}
	return out
}
