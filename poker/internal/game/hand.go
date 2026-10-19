package game

import "sort"

// Categorias de mano, de menor a mayor valor.
const (
	HighCard = iota
	OnePair
	TwoPair
	ThreeOfAKind
	Straight
	Flush
	FullHouse
	FourOfAKind
	StraightFlush
)

// HandRank es el resultado comparable de evaluar una mano. Kickers es la
// lista ordenada de valores que desempata dentro de la misma categoria.
type HandRank struct {
	Category int
	Kickers  []int
}

// Compare devuelve >0 si a gana, <0 si b gana, 0 si empatan.
func (a HandRank) Compare(b HandRank) int {
	if a.Category != b.Category {
		return a.Category - b.Category
	}
	for i := 0; i < len(a.Kickers) && i < len(b.Kickers); i++ {
		if a.Kickers[i] != b.Kickers[i] {
			return a.Kickers[i] - b.Kickers[i]
		}
	}
	return 0
}

// handNames traduce la categoria de una mano a su nombre en espanol.
var handNames = map[int]string{
	HighCard:      "Carta alta",
	OnePair:       "Par",
	TwoPair:       "Doble par",
	ThreeOfAKind:  "Trio",
	Straight:      "Escalera",
	Flush:         "Color",
	FullHouse:     "Full",
	FourOfAKind:   "Poker",
	StraightFlush: "Escalera de color",
}

// Name devuelve el nombre de la categoria de la mano.
func (r HandRank) Name() string {
	if r.Category < 0 {
		return ""
	}
	if name, ok := handNames[r.Category]; ok {
		return name
	}
	return ""
}

// BestHand evalua la mejor mano de 5 cartas posible entre todas las cartas
// dadas (normalmente 7: 2 privadas + 5 comunitarias).
func BestHand(cards []Card) HandRank {
	if len(cards) < 5 {
		return HandRank{Category: -1}
	}
	best := HandRank{Category: -1}
	indices := make([]int, 5)
	var rec func(start, depth int)
	rec = func(start, depth int) {
		if depth == 5 {
			combo := make([]Card, 5)
			for i, idx := range indices {
				combo[i] = cards[idx]
			}
			r := evaluate5(combo)
			if best.Category < 0 || r.Compare(best) > 0 {
				best = r
			}
			return
		}
		for i := start; i <= len(cards)-(5-depth); i++ {
			indices[depth] = i
			rec(i+1, depth+1)
		}
	}
	rec(0, 0)
	return best
}

// evaluate5 evalua exactamente cinco cartas.
func evaluate5(cards []Card) HandRank {
	ranks := make([]int, len(cards))
	for i, c := range cards {
		ranks[i] = rankValue(c.Rank)
	}
	sort.Sort(sort.Reverse(sort.IntSlice(ranks)))

	flush := true
	for i := 1; i < len(cards); i++ {
		if cards[i].Suit != cards[0].Suit {
			flush = false
			break
		}
	}

	straight, high := isStraight(ranks)
	if flush && straight {
		return HandRank{Category: StraightFlush, Kickers: []int{high}}
	}

	type group struct{ rank, count int }
	counts := map[int]int{}
	for _, r := range ranks {
		counts[r]++
	}
	groups := make([]group, 0, len(counts))
	for r, c := range counts {
		groups = append(groups, group{r, c})
	}
	sort.Slice(groups, func(i, j int) bool {
		if groups[i].count != groups[j].count {
			return groups[i].count > groups[j].count
		}
		return groups[i].rank > groups[j].rank
	})

	switch {
	case groups[0].count == 4:
		return HandRank{Category: FourOfAKind, Kickers: []int{groups[0].rank, groups[1].rank}}
	case groups[0].count == 3 && groups[1].count == 2:
		return HandRank{Category: FullHouse, Kickers: []int{groups[0].rank, groups[1].rank}}
	case flush:
		return HandRank{Category: Flush, Kickers: ranks}
	case straight:
		return HandRank{Category: Straight, Kickers: []int{high}}
	case groups[0].count == 3:
		kickers := []int{groups[0].rank}
		for _, g := range groups[1:] {
			kickers = append(kickers, g.rank)
		}
		return HandRank{Category: ThreeOfAKind, Kickers: kickers}
	case groups[0].count == 2 && groups[1].count == 2:
		hi, lo := groups[0].rank, groups[1].rank
		if hi < lo {
			hi, lo = lo, hi
		}
		return HandRank{Category: TwoPair, Kickers: []int{hi, lo, groups[2].rank}}
	case groups[0].count == 2:
		kickers := []int{groups[0].rank}
		for _, g := range groups[1:] {
			kickers = append(kickers, g.rank)
		}
		return HandRank{Category: OnePair, Kickers: kickers}
	default:
		return HandRank{Category: HighCard, Kickers: ranks}
	}
}

// isStraight detecta una escalera en valores ordenados descendentemente y
// devuelve la carta alta. Soporta la escalera "wheel" (A-5-4-3-2), cuya alta
// es el 5.
func isStraight(ranks []int) (bool, int) {
	uniq := make([]int, 0, len(ranks))
	for i, r := range ranks {
		if i == 0 || r != ranks[i-1] {
			uniq = append(uniq, r)
		}
	}
	if len(uniq) < 5 {
		return false, 0
	}
	if uniq[0]-uniq[4] == 4 {
		return true, uniq[0]
	}
	if uniq[0] == 14 && uniq[1] == 5 && uniq[2] == 4 && uniq[3] == 3 && uniq[4] == 2 {
		return true, 5
	}
	return false, 0
}
