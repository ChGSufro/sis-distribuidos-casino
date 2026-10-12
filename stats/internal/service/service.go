package service

import (
	"context"
	"encoding/json"
	"sort"
	"time"

	statsPb "github.com/ChGSufro/sis-distribuidos-casino/stats/pb"
	"github.com/ChGSufro/sis-distribuidos-casino/stats/internal/agg"
	"github.com/ChGSufro/sis-distribuidos-casino/stats/internal/client"
	"github.com/ChGSufro/sis-distribuidos-casino/stats/internal/store"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	DefaultLeaderboardLimit = 10
	MaxLeaderboardLimit     = 100
)

type Service struct {
	store *store.MongoStore
}

func New(s *store.MongoStore) *Service {
	return &Service{store: s}
}

var _ client.Handler = (*Service)(nil)

// OnEvent procesa un evento del stream: agrega player_stats, player_game_stats
// y persiste. Cualquier error detiene el avance del cursor (no se pierde nada).
func (s *Service) OnEvent(ctx context.Context, ev *statsPb.GameEvent) error {
	details := map[string]any{}
	if ev.Details != "" {
		if err := json.Unmarshal([]byte(ev.Details), &details); err != nil {
			details = nil
		}
	}
	ts, err := time.Parse(time.RFC3339, ev.Timestamp)
	if err != nil {
		ts = time.Now().UTC()
	}

	cur, err := s.store.GetPlayerStats(ctx, ev.UserId)
	if err != nil {
		return err
	}
	if cur == nil {
		cur = &agg.PlayerStatsDoc{UserID: ev.UserId}
	}

	updated, m := agg.Apply(*cur, agg.Event{
		Action:    ev.Action,
		Timestamp: ts,
		Details:   details,
	})
	if err := s.store.UpsertPlayerStats(ctx, &updated); err != nil {
		return err
	}

	if m.WageredCents != 0 || m.WonCents != 0 || m.Played {
		if err := s.store.ApplyPlayerGameStats(ctx, ev.UserId, ev.GameType, m); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) GetPlayerStats(ctx context.Context, req *statsPb.GetPlayerStatsRequest) (*statsPb.GetPlayerStatsResponse, error) {
	if req.UserId == "" {
		return nil, status.Error(codes.InvalidArgument, "user_id requerido")
	}
	doc, err := s.store.GetPlayerStats(ctx, req.UserId)
	if err != nil {
		return nil, status.Errorf(codes.Unavailable, "consultar stats: %v", err)
	}
	if doc == nil {
		return &statsPb.GetPlayerStatsResponse{UserId: req.UserId}, nil
	}

	return &statsPb.GetPlayerStatsResponse{
		UserId:            doc.UserID,
		TotalWageredCents: doc.TotalWageredCents,
		TotalWonCents:     doc.TotalWonCents,
		NetProfitCents:    doc.NetProfitCents,
		GamesPlayed:       int32(doc.GamesPlayed),
		WinRate:           doc.WinRate,
		LargestWinCents:   doc.LargestWinCents,
		CurrentStreak:     int32(doc.CurrentStreak),
		BestStreak:        int32(doc.BestStreak),
		LastUpdated:       isoOrEmpty(doc.LastUpdated),
	}, nil
}

func (s *Service) GetPlayerGameStats(ctx context.Context, req *statsPb.GetPlayerGameStatsRequest) (*statsPb.GetPlayerGameStatsResponse, error) {
	if req.UserId == "" {
		return nil, status.Error(codes.InvalidArgument, "user_id requerido")
	}
	if !validGameType(req.GameType) {
		return nil, status.Error(codes.InvalidArgument, "game_type debe ser blackjack o poker")
	}
	doc, err := s.store.GetPlayerGameStats(ctx, req.UserId, req.GameType)
	if err != nil {
		return nil, status.Errorf(codes.Unavailable, "consultar stats de juego: %v", err)
	}
	if doc == nil {
		return &statsPb.GetPlayerGameStatsResponse{UserId: req.UserId, GameType: req.GameType}, nil
	}

	return &statsPb.GetPlayerGameStatsResponse{
		UserId:            doc.UserID,
		GameType:          doc.GameType,
		GamesPlayed:       int32(doc.GamesPlayed),
		TotalWageredCents: doc.TotalWageredCents,
		TotalWonCents:     doc.TotalWonCents,
		LargestWinCents:   doc.LargestWinCents,
		LastUpdated:       isoOrEmpty(doc.LastUpdated),
	}, nil
}

func (s *Service) GetLeaderboard(ctx context.Context, req *statsPb.GetLeaderboardRequest) (*statsPb.GetLeaderboardResponse, error) {
	p := agg.Period(req.Period)
	if p == "" {
		p = agg.AllTime
	}
	if !p.Valid() {
		return nil, status.Errorf(codes.InvalidArgument, "period debe ser daily, weekly, monthly o all_time")
	}
	limit := int(req.Limit)
	if limit <= 0 {
		limit = DefaultLeaderboardLimit
	}
	if limit > MaxLeaderboardLimit {
		limit = MaxLeaderboardLimit
	}

	now := time.Now().UTC()
	entries, err := s.computeLeaderboard(ctx, p, now, limit)
	if err != nil {
		return nil, status.Errorf(codes.Unavailable, "calcular leaderboard: %v", err)
	}

	var start *time.Time
	if p != agg.AllTime {
		st := agg.StartOf(p, now)
		start = &st
	}
	snapshot := &store.RankingDoc{
		Period:      string(p),
		PeriodStart: start,
		Entries:     entries,
		ComputedAt:  now,
	}
	if err := s.store.SaveRanking(ctx, snapshot); err != nil {
		return nil, status.Errorf(codes.Unavailable, "persistir ranking: %v", err)
	}

	resp := &statsPb.GetLeaderboardResponse{
		Period:      string(p),
		PeriodStart: isoOrEmptyTime(start),
		ComputedAt:  now.Format(time.RFC3339),
	}
	for i := range entries {
		resp.Entries = append(resp.Entries, &statsPb.LeaderboardEntry{
			Rank:        int32(entries[i].Rank),
			UserId:      entries[i].UserID,
			Username:    "", // enriquecimiento del Gateway (pendiente)
			ScoreCents:  entries[i].ScoreCents,
			GamesPlayed: int32(entries[i].GamesPlayed),
		})
	}
	return resp, nil
}

func (s *Service) computeLeaderboard(ctx context.Context, p agg.Period, now time.Time, limit int) ([]store.RankingEntry, error) {
	docs, err := s.store.AllPlayerStats(ctx)
	if err != nil {
		return nil, err
	}

	type row struct {
		user    string
		score   int64
		games   int
	}
	rows := make([]row, 0, len(docs))

	if p == agg.AllTime {
		for i := range docs {
			rows = append(rows, row{docs[i].UserID, docs[i].NetProfitCents, docs[i].GamesPlayed})
		}
	} else {
		key := agg.KeyFor(p, now)
		for i := range docs {
			b, ok := docs[i].Periods[key]
			if !ok {
				continue
			}
			rows = append(rows, row{docs[i].UserID, b.WonCents - b.WageredCents, b.Games})
		}
	}

	sort.SliceStable(rows, func(i, j int) bool {
		if rows[i].score != rows[j].score {
			return rows[i].score > rows[j].score
		}
		return rows[i].user < rows[j].user
	})

	if len(rows) > limit {
		rows = rows[:limit]
	}

	out := make([]store.RankingEntry, 0, len(rows))
	for i, r := range rows {
		out = append(out, store.RankingEntry{
			UserID:      r.user,
			Rank:        i + 1,
			ScoreCents:  r.score,
			GamesPlayed: r.games,
		})
	}
	return out, nil
}

func validGameType(g string) bool {
	return g == "blackjack" || g == "poker"
}

func isoOrEmpty(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}

func isoOrEmptyTime(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}