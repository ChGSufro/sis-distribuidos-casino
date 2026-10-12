package game

import (
	"math/rand"
	"testing"
)

func TestNewDeckHas52UniqueCards(t *testing.T) {
	deck := NewDeck()
	if len(deck) != 52 {
		t.Fatalf("el mazo debe tener 52 cartas, tiene %d", len(deck))
	}
	seen := map[Card]bool{}
	for _, c := range deck {
		if seen[c] {
			t.Fatalf("carta duplicada: %+v", c)
		}
		seen[c] = true
	}
}

func TestShuffleAndDeal(t *testing.T) {
	deck := NewDeck()
	rng := rand.New(rand.NewSource(1))
	Shuffle(deck, rng)

	hand, rest := Deal(deck, 2)
	if len(hand) != 2 {
		t.Fatalf("se esperaban 2 cartas, hay %d", len(hand))
	}
	if len(rest) != 50 {
		t.Fatalf("el mazo restante debe tener 50, tiene %d", len(rest))
	}
	for _, c := range hand {
		for _, r := range rest {
			if c == r {
				t.Fatalf("carta %+v repartida y aun en el mazo", c)
			}
		}
	}
}
