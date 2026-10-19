package service

import (
	"time"

	"github.com/ChGSufro/sis-distribuidos-casino/poker/internal/game"
	pb "github.com/ChGSufro/sis-distribuidos-casino/poker/pb"
)

func cardToPB(c game.Card) *pb.Card {
	return &pb.Card{Suit: c.Suit, Rank: c.Rank}
}

func cardsToPB(cards []game.Card) []*pb.Card {
	out := make([]*pb.Card, 0, len(cards))
	for _, c := range cards {
		out = append(out, cardToPB(c))
	}
	return out
}

func tableToPB(t *game.Table) *pb.Table {
	current := int32(0)
	for _, s := range t.Seats {
		if s.Status == game.SeatActive {
			current++
		}
	}
	return &pb.Table{
		TableId:         t.ID,
		Name:            t.Name,
		SmallBlindCents: t.SmallBlindCents,
		BigBlindCents:   t.BigBlindCents,
		MaxPlayers:      t.MaxPlayers,
		CurrentPlayers:  current,
		Status:          t.Status,
		CreatedBy:       t.CreatedBy,
	}
}

func seatToPB(s game.Seat) *pb.Seat {
	return &pb.Seat{
		SeatPosition: s.SeatPosition,
		UserId:       s.UserID,
		StackCents:   s.StackCents,
		Status:       s.Status,
	}
}

func seatsToPB(t *game.Table) []*pb.Seat {
	out := make([]*pb.Seat, 0, len(t.Seats))
	for _, s := range t.Seats {
		out = append(out, seatToPB(s))
	}
	return out
}

// livePot calcula el bote de la ronda sumando las apuestas de todos los
// jugadores. Funciona en cualquier etapa: al cerrar la ronda coincide con
// el PotSizeCents que asienta el motor.
func livePot(round *game.Round) int64 {
	total := int64(0)
	for i := range round.Players {
		total += round.Players[i].TotalBetInRoundCents
	}
	return total
}

func playersToPB(round *game.Round) []*pb.PlayerState {
	out := make([]*pb.PlayerState, 0, len(round.Players))
	for i := range round.Players {
		p := &round.Players[i]
		out = append(out, &pb.PlayerState{
			UserId:               p.UserID,
			SeatPosition:         p.SeatPosition,
			StackCents:           p.StackCents,
			TotalBetInRoundCents: p.TotalBetInRoundCents,
			Status:               p.Status,
			StreetBetCents:       p.StreetBetCents,
		})
	}
	return out
}

// revealsFor devuelve las cartas privadas de los contendientes, solo
// cuando la ronda termino en showdown (nunca en abandono sin mostrar).
func revealsFor(round *game.Round) []*pb.PlayerReveal {
	if round.EndStage != game.StageShowdown {
		return nil
	}
	out := make([]*pb.PlayerReveal, 0, len(round.Players))
	for i := range round.Players {
		p := &round.Players[i]
		if p.Status == game.PlayerFolded || len(p.HoleCards) == 0 {
			continue
		}
		cards := make([]game.Card, 0, len(p.HoleCards)+len(round.CommunityCards))
		cards = append(cards, p.HoleCards...)
		cards = append(cards, round.CommunityCards...)
		out = append(out, &pb.PlayerReveal{
			UserId:       p.UserID,
			SeatPosition: p.SeatPosition,
			HoleCards:    cardsToPB(p.HoleCards),
			HandName:     game.BestHand(cards).Name(),
		})
	}
	return out
}

func roundStateFor(round *game.Round, userID string) *pb.RoundState {
	state := &pb.RoundState{
		RoundId:         round.ID,
		Stage:           round.Stage,
		CommunityCards:  cardsToPB(round.CommunityCards),
		PotSizeCents:    livePot(round),
		CurrentBetCents: round.CurrentBetCents,
		CurrentTurn:     round.CurrentTurn,
		TurnExpiresAt:   round.TurnExpiresAt.Format(time.RFC3339),
		DealerPosition:  round.DealerPosition,
		Actions:         actionsToPB(round.Actions),
		SidePots:        sidePotsToPB(round.SidePots),
		Players:         playersToPB(round),
		Reveals:         revealsFor(round),
	}
	if p := round.PlayerByUser(userID); p != nil {
		state.YourHoleCards = cardsToPB(p.HoleCards)
	}
	if round.IsSettled() {
		state.Winners = winnersToPB(round.Winners)
	}
	state.LastRaiseCents = round.LastRaiseCents
	state.MinRaiseToCents = round.MinRaiseToCents()
	return state
}

func tableUpdate(round *game.Round) *pb.TableUpdate {
	update := &pb.TableUpdate{
		RoundId:         round.ID,
		Stage:           round.Stage,
		CurrentTurn:     round.CurrentTurn,
		CurrentBetCents: round.CurrentBetCents,
		PotSizeCents:    livePot(round),
		CommunityCards:  cardsToPB(round.CommunityCards),
		Players:         playersToPB(round),
		TurnExpiresAt:   round.TurnExpiresAt.Format(time.RFC3339),
		DealerPosition:  round.DealerPosition,
		Actions:         actionsToPB(round.Actions),
		SidePots:        sidePotsToPB(round.SidePots),
		Reveals:         revealsFor(round),
	}
	update.LastRaiseCents = round.LastRaiseCents
	update.MinRaiseToCents = round.MinRaiseToCents()
	if round.IsSettled() {
		update.Winners = winnersToPB(round.Winners)
	}
	return update
}
func actionsToPB(actions []game.Action) []*pb.Action {
	out := make([]*pb.Action, 0, len(actions))
	for _, a := range actions {
		out = append(out, &pb.Action{
			UserId:      a.UserID,
			Action:      a.Action,
			AmountCents: a.AmountCents,
			Timestamp:   a.Timestamp.Format(time.RFC3339),
		})
	}
	return out
}

func sidePotsToPB(pots []game.SidePot) []*pb.SidePot {
	out := make([]*pb.SidePot, 0, len(pots))
	for _, p := range pots {
		out = append(out, &pb.SidePot{AmountCents: p.AmountCents, EligiblePlayers: p.EligiblePlayers})
	}
	return out
}

func winnersToPB(winners []game.Winner) []*pb.Winner {
	out := make([]*pb.Winner, 0, len(winners))
	for _, w := range winners {
		out = append(out, &pb.Winner{UserId: w.UserID, PotType: w.PotType, AmountCents: w.AmountCents})
	}
	return out
}
