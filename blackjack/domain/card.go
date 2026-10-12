package domain

import (
	"crypto/rand"
	"math/big"
)

type Suit string

const (
	SuitHearts   Suit = "H"
	SuitDiamonds Suit = "D"
	SuitClubs    Suit = "C"
	SuitSpades   Suit = "S"
)

type Rank string

const (
	Rank2     Rank = "2"
	Rank3     Rank = "3"
	Rank4     Rank = "4"
	Rank5     Rank = "5"
	Rank6     Rank = "6"
	Rank7     Rank = "7"
	Rank8     Rank = "8"
	Rank9     Rank = "9"
	Rank10    Rank = "10"
	RankJack  Rank = "J"
	RankQueen Rank = "Q"
	RankKing  Rank = "K"
	RankAce   Rank = "A"
)

type Card struct {
	Suit   Suit `bson:"suit" json:"suit"`
	Rank   Rank `bson:"rank" json:"rank"`
	Value  int  `bson:"value" json:"value"`
	Hidden bool `bson:"hidden" json:"hidden"`
}

func NewCard(suit Suit, rank Rank, hidden bool) Card {
	val := rankValue(rank)
	return Card{
		Suit:   suit,
		Rank:   rank,
		Value:  val,
		Hidden: hidden,
	}
}

func rankValue(rank Rank) int {
	switch rank {
	case RankAce:
		return 11
	case RankKing, RankQueen, RankJack, Rank10:
		return 10
	case Rank9:
		return 9
	case Rank8:
		return 8
	case Rank7:
		return 7
	case Rank6:
		return 6
	case Rank5:
		return 5
	case Rank4:
		return 4
	case Rank3:
		return 3
	case Rank2:
		return 2
	default:
		return 0
	}
}

// CalculateHandValue calcula el puntaje óptimo de una mano considerando Ases (11 o 1).
func CalculateHandValue(cards []Card) int {
	total := 0
	aceCount := 0

	for _, card := range cards {
		if card.Rank == RankAce {
			aceCount++
			total += 11
		} else {
			total += card.Value
		}
	}

	// Reducir Ases de 11 a 1 si el total supera 21
	for total > 21 && aceCount > 0 {
		total -= 10
		aceCount--
	}

	return total
}

// IsBlackjack verifica si exactamente 2 cartas conforman un Blackjack natural (21 puntos).
func IsBlackjack(cards []Card) bool {
	return len(cards) == 2 && CalculateHandValue(cards) == 21
}

// Deck representa una baraja o shoe de Blackjack.
type Deck struct {
	cards []Card
}

func NewStandardDeck(numDecks int) *Deck {
	if numDecks <= 0 {
		numDecks = 1
	}
	suits := []Suit{SuitHearts, SuitDiamonds, SuitClubs, SuitSpades}
	ranks := []Rank{Rank2, Rank3, Rank4, Rank5, Rank6, Rank7, Rank8, Rank9, Rank10, RankJack, RankQueen, RankKing, RankAce}

	var cards []Card
	for i := 0; i < numDecks; i++ {
		for _, s := range suits {
			for _, r := range ranks {
				cards = append(cards, NewCard(s, r, false))
			}
		}
	}

	d := &Deck{cards: cards}
	d.Shuffle()
	return d
}

// Shuffle mezcla las cartas utilizando aleatoriedad criptográficamente segura.
func (d *Deck) Shuffle() {
	n := len(d.cards)
	for i := n - 1; i > 0; i-- {
		randIdxBig, err := rand.Int(rand.Reader, big.NewInt(int64(i+1)))
		if err != nil {
			continue
		}
		j := int(randIdxBig.Int64())
		d.cards[i], d.cards[j] = d.cards[j], d.cards[i]
	}
}

// Draw extrae la primera carta del mazo.
func (d *Deck) Draw(hidden bool) Card {
	if len(d.cards) == 0 {
		*d = *NewStandardDeck(1)
	}
	card := d.cards[0]
	card.Hidden = hidden
	d.cards = d.cards[1:]
	return card
}

