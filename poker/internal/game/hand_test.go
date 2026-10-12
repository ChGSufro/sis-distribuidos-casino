package game

import "testing"

func mustCards(t *testing.T, spec string) []Card {
	t.Helper()
	cards := make([]Card, 0)
	i := 0
	for i < len(spec) {
		rank := string(spec[i])
		i++
		if i < len(spec) && spec[i] == '0' {
			rank += "0"
			i++
		}
		if i >= len(spec) {
			t.Fatalf("especificacion de carta invalida: %q", spec)
		}
		suit := string(spec[i])
		i++
		cards = append(cards, Card{Suit: suit, Rank: rank})
	}
	return cards
}

func TestCategoryOrdering(t *testing.T) {
	sf := BestHand(mustCards(t, "9H8H7H6H5H"))
	quads := BestHand(mustCards(t, "ASADAHAC2D"))
	full := BestHand(mustCards(t, "KSKDKH2S2D"))
	flush := BestHand(mustCards(t, "AHQH9H6H3H"))
	straight := BestHand(mustCards(t, "9S8D7C6H5S"))
	trips := BestHand(mustCards(t, "5S5D5C9H2D"))
	twoPair := BestHand(mustCards(t, "ASAD7H7C2D"))
	onePair := BestHand(mustCards(t, "ASADKHQC2D"))
	high := BestHand(mustCards(t, "ASKDQH9C3D"))

	ordered := []HandRank{high, onePair, twoPair, trips, straight, flush, full, quads, sf}
	for i := 1; i < len(ordered); i++ {
		if ordered[i-1].Compare(ordered[i]) >= 0 {
			t.Fatalf("orden invalido en posicion %d: %+v vs %+v", i, ordered[i-1], ordered[i])
		}
	}
}

func TestWheelStraight(t *testing.T) {
	wheel := BestHand(mustCards(t, "AS2D3C4H5S"))
	if wheel.Category != Straight {
		t.Fatalf("se esperaba escalera, se obtuvo categoria %d", wheel.Category)
	}
	if wheel.Kickers[0] != 5 {
		t.Fatalf("la carta alta del wheel debe ser 5, fue %d", wheel.Kickers[0])
	}
	sixHigh := BestHand(mustCards(t, "2S3D4C5H6S"))
	if wheel.Compare(sixHigh) >= 0 {
		t.Fatal("el wheel debe perder contra la escalera al 6")
	}
}

func TestKickersBreakTies(t *testing.T) {
	aceKicker := BestHand(mustCards(t, "QSQD2H3C4DAH"))
	kingKicker := BestHand(mustCards(t, "QCQH2D3S4CKD"))
	if aceKicker.Compare(kingKicker) <= 0 {
		t.Fatal("la pareja de Q con A kicker debe ganar a la de K kicker")
	}

	flushAce := BestHand(mustCards(t, "AHQH9H6H3H"))
	flushKing := BestHand(mustCards(t, "KHQH9H6H3H"))
	if flushAce.Compare(flushKing) <= 0 {
		t.Fatal("el flush con A alto debe ganar al flush con K alto")
	}
}

func TestBestOfSeven(t *testing.T) {
	cards := mustCards(t, "ASKSQSJS10S2D3H")
	best := BestHand(cards)
	if best.Category != StraightFlush {
		t.Fatalf("se esperaba escalera de color, se obtuvo %d", best.Category)
	}
	if best.Kickers[0] != 14 {
		t.Fatalf("la carta alta esperada era A (14), fue %d", best.Kickers[0])
	}
}

func TestTieIsZero(t *testing.T) {
	a := BestHand(mustCards(t, "AHKDQCJS10H"))
	b := BestHand(mustCards(t, "ASKDQCJS10D"))
	if a.Compare(b) != 0 {
		t.Fatal("dos escaleras iguales al A deben empatar")
	}
}
