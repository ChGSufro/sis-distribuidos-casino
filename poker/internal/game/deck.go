package game

import "math/rand"

// ranks en orden ascendente de valor.
var ranks = []string{"2", "3", "4", "5", "6", "7", "8", "9", "10", "J", "Q", "K", "A"}

// suits de la baraja.
var suits = []string{SuitHearts, SuitDiamonds, SuitClubs, SuitSpades}

// NewDeck devuelve una baraja ordenada de 52 cartas.
func NewDeck() []Card {
	deck := make([]Card, 0, len(ranks)*len(suits))
	for _, s := range suits {
		for _, r := range ranks {
			deck = append(deck, Card{Suit: s, Rank: r})
		}
	}
	return deck
}

// Shuffle baraja el mazo in-place usando el generador dado.
func Shuffle(deck []Card, rng *rand.Rand) {
	rng.Shuffle(len(deck), func(i, j int) {
		deck[i], deck[j] = deck[j], deck[i]
	})
}

// Deal reparte n cartas del tope del mazo, devolviendo las cartas y el mazo
// restante. Si no hay suficientes cartas devuelve el mazo sin modificar.
func Deal(deck []Card, n int) ([]Card, []Card) {
	if n < 0 || n > len(deck) {
		return nil, deck
	}
	hand := make([]Card, n)
	copy(hand, deck[:n])
	rest := make([]Card, len(deck)-n)
	copy(rest, deck[n:])
	return hand, rest
}

// rankValue devuelve el valor numerico de un rango (2..14).
func rankValue(rank string) int {
	switch rank {
	case "J":
		return 11
	case "Q":
		return 12
	case "K":
		return 13
	case "A":
		return 14
	default:
		switch rank {
		case "2":
			return 2
		case "3":
			return 3
		case "4":
			return 4
		case "5":
			return 5
		case "6":
			return 6
		case "7":
			return 7
		case "8":
			return 8
		case "9":
			return 9
		case "10":
			return 10
		}
	}
	return 0
}
