package agg

import (
	"testing"
	"time"
)

func ts(s string) time.Time {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		panic(err)
	}
	return t
}

func handResult(user string) Event {
	return Event{
		Action:    "hand_result",
		Timestamp: ts("2026-10-12T10:00:00Z"),
		Details: map[string]any{
			"betAmountCents": float64(1000),
			"result":         "win",
			"winAmountCents": float64(2000),
		},
	}
}

func TestApplyHandResultWin(t *testing.T) {
	stats, m := Apply(PlayerStatsDoc{UserID: "u-1"}, handResult("u-1"))

	if m.WageredCents != 1000 || m.WonCents != 2000 || !m.Played || m.Result != ResultWin {
		t.Fatalf("metrics inesperadas: %+v", m)
	}
	if stats.GamesPlayed != 1 || stats.Wins != 1 {
		t.Fatalf("games/wins: %d/%d", stats.GamesPlayed, stats.Wins)
	}
	if stats.TotalWageredCents != 1000 || stats.TotalWonCents != 2000 {
		t.Fatalf("totales: %d/%d", stats.TotalWageredCents, stats.TotalWonCents)
	}
	if stats.NetProfitCents != 1000 {
		t.Fatalf("netProfit: %d", stats.NetProfitCents)
	}
	if stats.CurrentStreak != 1 || stats.BestStreak != 1 {
		t.Fatalf("streak: %d/%d", stats.CurrentStreak, stats.BestStreak)
	}
	if stats.WinRate != 1.0 {
		t.Fatalf("winRate: %f", stats.WinRate)
	}
	if stats.LargestWinCents != 2000 {
		t.Fatalf("largestWin: %d", stats.LargestWinCents)
	}
	if stats.Periods["daily:2026-10-12"].WageredCents != 1000 || stats.Periods["daily:2026-10-12"].Games != 1 {
		t.Fatalf("bucket diario: %+v", stats.Periods["daily:2026-10-12"])
	}
}

func TestApplyStreakTransitions(t *testing.T) {
	win := func() Event {
		e := handResult("u-1")
		e.Details = map[string]any{"betAmountCents": float64(100), "result": "win", "winAmountCents": float64(200)}
		return e
	}
	lose := func() Event {
		e := handResult("u-1")
		e.Details = map[string]any{"betAmountCents": float64(100), "result": "lose"}
		return e
	}

	stats := PlayerStatsDoc{UserID: "u-1"}
	stats, _ = Apply(stats, win())
	stats, _ = Apply(stats, win())
	stats, _ = Apply(stats, lose())
	stats, _ = Apply(stats, win())

	if stats.CurrentStreak != 1 {
		t.Fatalf("currentStreak: %d", stats.CurrentStreak)
	}
	if stats.BestStreak != 2 {
		t.Fatalf("bestStreak: %d", stats.BestStreak)
	}
	if stats.GamesPlayed != 4 || stats.Wins != 3 {
		t.Fatalf("games/wins: %d/%d", stats.GamesPlayed, stats.Wins)
	}
}

func TestApplyPushIsNeutral(t *testing.T) {
	e := handResult("u-1")
	e.Details = map[string]any{"betAmountCents": float64(500), "result": "push"}
	stats, m := Apply(PlayerStatsDoc{UserID: "u-1"}, e)

	if m.Result != ResultNeutral || m.WonCents != 0 {
		t.Fatalf("metrics: %+v", m)
	}
	if stats.TotalWageredCents != 500 {
		t.Fatalf("wagered: %d", stats.TotalWageredCents)
	}
	if stats.CurrentStreak != 0 {
		t.Fatalf("streak deberia ser neutro: %d", stats.CurrentStreak)
	}
}

func TestApplyPlayerBuyin(t *testing.T) {
	buyin := Event{
		Action:    "player_buyin",
		Timestamp: ts("2026-10-12T11:30:00Z"),
		Details:   map[string]any{"amountCents": float64(5000)},
	}
	stats, m := Apply(PlayerStatsDoc{UserID: "u-1"}, buyin)

	if m.WageredCents != 5000 || m.Played || m.Result != ResultNeutral {
		t.Fatalf("metrics: %+v", m)
	}
	if stats.TotalWageredCents != 5000 || stats.GamesPlayed != 0 {
		t.Fatalf("totales: %d/%d", stats.TotalWageredCents, stats.GamesPlayed)
	}
}

func TestApplyTableResult(t *testing.T) {
	table := Event{
		Action:    "table_result",
		Timestamp: ts("2026-10-12T12:00:00Z"),
		Details: map[string]any{
			"yourResult":     "win",
			"winAmountCents": float64(3500),
		},
	}
	stats, m := Apply(PlayerStatsDoc{UserID: "u-1"}, table)

	if m.Played != true || m.Result != ResultWin || m.WonCents != 3500 {
		t.Fatalf("metrics: %+v", m)
	}
	if stats.GamesPlayed != 1 || stats.CurrentStreak != 1 {
		t.Fatalf("games/streak: %d/%d", stats.GamesPlayed, stats.CurrentStreak)
	}
}

func TestKeyForPeriods(t *testing.T) {
	now := ts("2026-10-12T15:04:05Z") // lunes de la semana ISO 2026-W42

	if k := KeyFor(Daily, now); k != "daily:2026-10-12" {
		t.Fatalf("daily: %s", k)
	}
	if k := KeyFor(Weekly, now); k != "weekly:2026-W42" {
		t.Fatalf("weekly: %s", k)
	}
	if k := KeyFor(Monthly, now); k != "monthly:2026-10" {
		t.Fatalf("monthly: %s", k)
	}
	if !StartOf(Daily, now).IsZero() && StartOf(Daily, now).Format("2006-01-02") != "2026-10-12" {
		t.Fatalf("start daily: %v", StartOf(Daily, now))
	}
}