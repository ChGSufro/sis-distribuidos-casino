package service

import (
	"time"

	pb "github.com/ChGSufro/sis-distribuidos-casino/poker/pb"
	"github.com/ChGSufro/sis-distribuidos-casino/poker/internal/game"
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

func roundStateFor(round *game.Round, userID string) *pb.RoundState {
	state := &pb.RoundState{
		RoundId:         round.ID,
		Stage:           round.Stage,
		CommunityCards:  cardsToPB(round.CommunityCards),
		PotSizeCents:    round.PotSizeCents,
		CurrentBetCents: round.CurrentBetCents,
		CurrentTurn:     round.CurrentTurn,
		TurnExpiresAt:   round.TurnExpiresAt.Format(time.RFC3339),
		DealerPosition:  round.DealerPosition,
		Actions:         actionsToPB(round.Actions),
		SidePots:        sidePotsToPB(round.SidePots),
	}
	if p := round.PlayerByUser(userID); p != nil {
		state.YourHoleCards = cardsToPB(p.HoleCards)
	}
	return state
}

func tableUpdate(round *game.Round) *pb.TableUpdate {
	players := make([]*pb.PlayerState, 0, len(round.Players))
	for _, p := range round.Players {
		players = append(players, &pb.PlayerState{
			UserId:               p.UserID,
			SeatPosition:         p.SeatPosition,
			StackCents:           p.StackCents,
			TotalBetInRoundCents: p.TotalBetInRoundCents,
			Status:               p.Status,
		})
	}
	update := &pb.TableUpdate{
		RoundId:         round.ID,
		Stage:           round.Stage,
		CurrentTurn:     round.CurrentTurn,
		CurrentBetCents: round.CurrentBetCents,
		PotSizeCents:    round.PotSizeCents,
		CommunityCards:  cardsToPB(round.CommunityCards),
		Players:         players,
		TurnExpiresAt:   round.TurnExpiresAt.Format(time.RFC3339),
		DealerPosition:  round.DealerPosition,
		Actions:         actionsToPB(round.Actions),
		SidePots:        sidePotsToPB(round.SidePots),
	}
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
