package agg

import (
	"time"
)

// Event es la representación normalizada de un evento del stream para la
// agregación. El paquete agg no depende de los protos (puro y testeable).
type Event struct {
	Action    string
	Timestamp time.Time
	Details   map[string]any
}

type Result int

const (
	ResultNeutral Result = iota
	ResultWin
	ResultLose
)

type Metrics struct {
	WageredCents int64
	WonCents     int64
	Played       bool
	Result       Result
}

type PeriodBucket struct {
	WageredCents int64 `bson:"wageredCents"`
	WonCents     int64 `bson:"wonCents"`
	Games        int   `bson:"games"`
}

type PlayerStatsDoc struct {
	UserID            string                    `bson:"userId"`
	TotalWageredCents int64                     `bson:"totalWageredCents"`
	TotalWonCents     int64                     `bson:"totalWonCents"`
	NetProfitCents    int64                     `bson:"netProfitCents"`
	GamesPlayed       int                       `bson:"gamesPlayed"`
	WinRate           float64                   `bson:"winRate"`
	LargestWinCents   int64                     `bson:"largestWinCents"`
	CurrentStreak     int                       `bson:"currentStreak"`
	BestStreak        int                       `bson:"bestStreak"`
	Wins              int                       `bson:"wins"`
	Periods           map[string]PeriodBucket   `bson:"periods"`
	LastUpdated       time.Time                 `bson:"lastUpdated"`
}

// Apply agrega un evento a las estadísticas del jugador y devuelve el doc
// actualizado junto con las métricas aplicadas (para player_game_stats).
func Apply(stats PlayerStatsDoc, ev Event) (PlayerStatsDoc, Metrics) {
	if stats.Periods == nil {
		stats.Periods = make(map[string]PeriodBucket)
	}
	stats.LastUpdated = ev.Timestamp.UTC()

	m := metricsFor(ev)

	stats.TotalWageredCents += m.WageredCents
	stats.TotalWonCents += m.WonCents
	if m.WonCents > stats.LargestWinCents {
		stats.LargestWinCents = m.WonCents
	}
	if m.Played {
		stats.GamesPlayed++
	}

	switch m.Result {
	case ResultWin:
		stats.Wins++
		if stats.CurrentStreak > 0 {
			stats.CurrentStreak++
		} else {
			stats.CurrentStreak = 1
		}
		if stats.CurrentStreak > stats.BestStreak {
			stats.BestStreak = stats.CurrentStreak
		}
	case ResultLose:
		if stats.CurrentStreak < 0 {
			stats.CurrentStreak--
		} else {
			stats.CurrentStreak = -1
		}
	}

	if stats.GamesPlayed > 0 {
		stats.WinRate = float64(stats.Wins) / float64(stats.GamesPlayed)
	}
	stats.NetProfitCents = stats.TotalWonCents - stats.TotalWageredCents

	for _, p := range []Period{Daily, Weekly, Monthly} {
		key := KeyFor(p, ev.Timestamp)
		b := stats.Periods[key]
		b.WageredCents += m.WageredCents
		b.WonCents += m.WonCents
		if m.Played {
			b.Games++
		}
		stats.Periods[key] = b
	}

	return stats, m
}

func metricsFor(ev Event) Metrics {
	var m Metrics

	switch ev.Action {
	case "hand_result":
		m.WageredCents = asCents(ev.Details, "betAmountCents")
		m.Played = true
		switch asString(ev.Details, "result") {
		case "win", "blackjack_win":
			m.Result = ResultWin
			m.WonCents = asCents(ev.Details, "winAmountCents")
		case "lose":
			m.Result = ResultLose
		}
	case "table_result":
		m.Played = true
		m.WageredCents = asCents(ev.Details, "betAmountCents")
		if asString(ev.Details, "yourResult") == "win" {
			m.Result = ResultWin
			m.WonCents = asCents(ev.Details, "winAmountCents")
		} else {
			m.Result = ResultLose
		}
	}

	return m
}

func asCents(details map[string]any, key string) int64 {
	if details == nil {
		return 0
	}
	switch v := details[key].(type) {
	case float64:
		return int64(v)
	case int64:
		return v
	}
	return 0
}

func asString(details map[string]any, key string) string {
	if details == nil {
		return ""
	}
	if v, ok := details[key].(string); ok {
		return v
	}
	return ""
}