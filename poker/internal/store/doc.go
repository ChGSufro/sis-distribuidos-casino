package store

import (
	"time"

	"github.com/ChGSufro/sis-distribuidos-casino/poker/internal/game"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type cardDoc struct {
	Suit string `bson:"suit"`
	Rank string `bson:"rank"`
}

type seatDoc struct {
	SeatPosition int32  `bson:"seatPosition"`
	UserID       string `bson:"userId"`
	StackCents   int64  `bson:"stackCents"`
	HoldID       string `bson:"holdId,omitempty"`
	Status       string `bson:"status"`
}

type tableDoc struct {
	ID              primitive.ObjectID `bson:"_id,omitempty"`
	Name            string             `bson:"name"`
	SmallBlindCents int64              `bson:"smallBlindCents"`
	BigBlindCents   int64              `bson:"bigBlindCents"`
	MaxPlayers      int32              `bson:"maxPlayers"`
	Status          string             `bson:"status"`
	Seats           []seatDoc          `bson:"seats"`
	CurrentRoundID  string             `bson:"currentRoundId,omitempty"`
	DealerPosition  int32              `bson:"dealerPosition"`
	CreatedAt       time.Time          `bson:"createdAt"`
}

type roundPlayerDoc struct {
	UserID               string    `bson:"userId"`
	SeatPosition         int32     `bson:"seatPosition"`
	HoleCards            []cardDoc `bson:"holeCards"`
	StackAtStartCents    int64     `bson:"stackAtStartCents"`
	StackCents           int64     `bson:"stackCents"`
	TotalBetInRoundCents int64     `bson:"totalBetInRoundCents"`
	StreetBetCents       int64     `bson:"streetBetCents"`
	HasActed             bool      `bson:"hasActed"`
	Status               string    `bson:"status"`
}

type sidePotDoc struct {
	AmountCents     int64    `bson:"amountCents"`
	EligiblePlayers []string `bson:"eligiblePlayers"`
}

type actionDoc struct {
	UserID      string    `bson:"userId"`
	Action      string    `bson:"action"`
	AmountCents int64     `bson:"amountCents"`
	Timestamp   time.Time `bson:"timestamp"`
}

type winnerDoc struct {
	UserID      string `bson:"userId"`
	PotType     string `bson:"potType"`
	AmountCents int64  `bson:"amountCents"`
}

type roundDoc struct {
	ID              primitive.ObjectID `bson:"_id,omitempty"`
	TableID         string             `bson:"tableId"`
	RoundNumber     int64              `bson:"roundNumber"`
	Players         []roundPlayerDoc   `bson:"players"`
	CommunityCards  []cardDoc          `bson:"communityCards"`
	PotSizeCents    int64              `bson:"potSizeCents"`
	SidePots        []sidePotDoc       `bson:"sidePots"`
	DealerPosition  int32              `bson:"dealerPosition"`
	CurrentTurn     int32              `bson:"currentTurn"`
	CurrentBetCents int64              `bson:"currentBetCents"`
	TurnExpiresAt   time.Time          `bson:"turnExpiresAt"`
	Stage           string             `bson:"stage"`
	Actions         []actionDoc        `bson:"actions"`
	Winners         []winnerDoc        `bson:"winners"`
	Deck            []cardDoc          `bson:"deck"`
	CreatedAt       time.Time          `bson:"createdAt"`
	SettledAt       *time.Time         `bson:"settledAt,omitempty"`
}

func cardsToDoc(cards []game.Card) []cardDoc {
	out := make([]cardDoc, 0, len(cards))
	for _, c := range cards {
		out = append(out, cardDoc{Suit: c.Suit, Rank: c.Rank})
	}
	return out
}

func cardsFromDoc(docs []cardDoc) []game.Card {
	out := make([]game.Card, 0, len(docs))
	for _, d := range docs {
		out = append(out, game.Card{Suit: d.Suit, Rank: d.Rank})
	}
	return out
}

func tableToDoc(t *game.Table) *tableDoc {
	seats := make([]seatDoc, 0, len(t.Seats))
	for _, s := range t.Seats {
		seats = append(seats, seatDoc{
			SeatPosition: s.SeatPosition,
			UserID:       s.UserID,
			StackCents:   s.StackCents,
			HoldID:       s.HoldID,
			Status:       s.Status,
		})
	}
	return &tableDoc{
		Name:            t.Name,
		SmallBlindCents: t.SmallBlindCents,
		BigBlindCents:   t.BigBlindCents,
		MaxPlayers:      t.MaxPlayers,
		Status:          t.Status,
		Seats:           seats,
		CurrentRoundID:  t.CurrentRoundID,
		DealerPosition:  t.DealerPosition,
		CreatedAt:       t.CreatedAt,
	}
}

func docToTable(d *tableDoc) *game.Table {
	seats := make([]game.Seat, 0, len(d.Seats))
	for _, s := range d.Seats {
		seats = append(seats, game.Seat{
			SeatPosition: s.SeatPosition,
			UserID:       s.UserID,
			StackCents:   s.StackCents,
			HoldID:       s.HoldID,
			Status:       s.Status,
		})
	}
	return &game.Table{
		ID:              d.ID.Hex(),
		Name:            d.Name,
		SmallBlindCents: d.SmallBlindCents,
		BigBlindCents:   d.BigBlindCents,
		MaxPlayers:      d.MaxPlayers,
		Status:          d.Status,
		Seats:           seats,
		CurrentRoundID:  d.CurrentRoundID,
		DealerPosition:  d.DealerPosition,
		CreatedAt:       d.CreatedAt,
	}
}

func roundToDoc(r *game.Round) *roundDoc {
	players := make([]roundPlayerDoc, 0, len(r.Players))
	for _, p := range r.Players {
		players = append(players, roundPlayerDoc{
			UserID:               p.UserID,
			SeatPosition:         p.SeatPosition,
			HoleCards:            cardsToDoc(p.HoleCards),
			StackAtStartCents:    p.StackAtStartCents,
			StackCents:           p.StackCents,
			TotalBetInRoundCents: p.TotalBetInRoundCents,
			StreetBetCents:       p.StreetBetCents,
			HasActed:             p.HasActed,
			Status:               p.Status,
		})
	}
	sidePots := make([]sidePotDoc, 0, len(r.SidePots))
	for _, sp := range r.SidePots {
		sidePots = append(sidePots, sidePotDoc{AmountCents: sp.AmountCents, EligiblePlayers: sp.EligiblePlayers})
	}
	actions := make([]actionDoc, 0, len(r.Actions))
	for _, a := range r.Actions {
		actions = append(actions, actionDoc{UserID: a.UserID, Action: a.Action, AmountCents: a.AmountCents, Timestamp: a.Timestamp})
	}
	winners := make([]winnerDoc, 0, len(r.Winners))
	for _, w := range r.Winners {
		winners = append(winners, winnerDoc{UserID: w.UserID, PotType: w.PotType, AmountCents: w.AmountCents})
	}
	return &roundDoc{
		TableID:         r.TableID,
		RoundNumber:     r.RoundNumber,
		Players:         players,
		CommunityCards:  cardsToDoc(r.CommunityCards),
		PotSizeCents:    r.PotSizeCents,
		SidePots:        sidePots,
		DealerPosition:  r.DealerPosition,
		CurrentTurn:     r.CurrentTurn,
		CurrentBetCents: r.CurrentBetCents,
		TurnExpiresAt:   r.TurnExpiresAt,
		Stage:           r.Stage,
		Actions:         actions,
		Winners:         winners,
		Deck:            cardsToDoc(r.Deck),
		CreatedAt:       r.CreatedAt,
		SettledAt:       r.SettledAt,
	}
}

func docToRound(d *roundDoc) *game.Round {
	players := make([]game.RoundPlayer, 0, len(d.Players))
	for _, p := range d.Players {
		players = append(players, game.RoundPlayer{
			UserID:               p.UserID,
			SeatPosition:         p.SeatPosition,
			HoleCards:            cardsFromDoc(p.HoleCards),
			StackAtStartCents:    p.StackAtStartCents,
			StackCents:           p.StackCents,
			TotalBetInRoundCents: p.TotalBetInRoundCents,
			StreetBetCents:       p.StreetBetCents,
			HasActed:             p.HasActed,
			Status:               p.Status,
		})
	}
	sidePots := make([]game.SidePot, 0, len(d.SidePots))
	for _, sp := range d.SidePots {
		sidePots = append(sidePots, game.SidePot{AmountCents: sp.AmountCents, EligiblePlayers: sp.EligiblePlayers})
	}
	actions := make([]game.Action, 0, len(d.Actions))
	for _, a := range d.Actions {
		actions = append(actions, game.Action{UserID: a.UserID, Action: a.Action, AmountCents: a.AmountCents, Timestamp: a.Timestamp})
	}
	winners := make([]game.Winner, 0, len(d.Winners))
	for _, w := range d.Winners {
		winners = append(winners, game.Winner{UserID: w.UserID, PotType: w.PotType, AmountCents: w.AmountCents})
	}
	return &game.Round{
		ID:              d.ID.Hex(),
		TableID:         d.TableID,
		RoundNumber:     d.RoundNumber,
		Players:         players,
		CommunityCards:  cardsFromDoc(d.CommunityCards),
		PotSizeCents:    d.PotSizeCents,
		SidePots:        sidePots,
		DealerPosition:  d.DealerPosition,
		CurrentTurn:     d.CurrentTurn,
		CurrentBetCents: d.CurrentBetCents,
		TurnExpiresAt:   d.TurnExpiresAt,
		Stage:           d.Stage,
		Actions:         actions,
		Winners:         winners,
		Deck:            cardsFromDoc(d.Deck),
		CreatedAt:       d.CreatedAt,
		SettledAt:       d.SettledAt,
	}
}
