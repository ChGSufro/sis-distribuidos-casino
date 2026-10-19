package router

import (
	"net/http"

	"github.com/ChGSufro/sis-distribuidos-casino/gateway/internal/client"
	"github.com/ChGSufro/sis-distribuidos-casino/gateway/internal/handler"
	"github.com/ChGSufro/sis-distribuidos-casino/gateway/internal/middleware"
)

func New(clients *client.Clients) http.Handler {
	mux := http.NewServeMux()

	authMW := middleware.NewAuthMiddleware(clients.Users)

	// Instanciar handlers
	authH := handler.NewAuthHandler(clients.Users, clients.Wallet)
	walletH := handler.NewWalletHandler(clients.Wallet)
	bjH := handler.NewBlackjackHandler(clients.Blackjack)
	pokerH := handler.NewPokerHandler(clients.Poker, clients.Users)
	historyH := handler.NewHistoryHandler(clients.History)
	statsH := handler.NewStatsHandler(clients.Stats)

	// Health check
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		handler.WriteJSON(w, http.StatusOK, map[string]string{
			"status":  "healthy",
			"service": "api-gateway",
		})
	})

	// ─── Rutas Públicas ──────────────────────────────────────
	mux.HandleFunc("POST /api/auth/register", authH.Register)
	mux.HandleFunc("POST /api/auth/login", authH.Login)
	mux.HandleFunc("POST /api/auth/refresh", authH.RefreshToken)
	mux.HandleFunc("GET /api/games/poker/tables", pokerH.ListTables)
	mux.HandleFunc("GET /api/stats/leaderboard", statsH.GetLeaderboard)

	// ─── Rutas Protegidas (JWT Requerido) ─────────────────────
	// Auth & Users
	mux.HandleFunc("POST /api/auth/logout", authMW.RequireAuth(authH.Logout))
	mux.HandleFunc("GET /api/auth/profile", authMW.RequireAuth(authH.GetProfile))
	mux.HandleFunc("GET /api/users/{id}/profile", authMW.RequireAuth(authH.GetProfile))

	// Wallet
	mux.HandleFunc("GET /api/wallet/balance", authMW.RequireAuth(walletH.GetBalance))
	mux.HandleFunc("POST /api/wallet/deposit", authMW.RequireAuth(walletH.Deposit))

	// Blackjack
	mux.HandleFunc("POST /api/games/blackjack/hands", authMW.RequireAuth(bjH.CreateHand))
	mux.HandleFunc("POST /api/games/blackjack/hands/{id}/action", authMW.RequireAuth(bjH.PlayerAction))
	mux.HandleFunc("GET /api/games/blackjack/hands/{id}", authMW.RequireAuth(bjH.GetHandState))

	// Poker
	mux.HandleFunc("POST /api/games/poker/tables", authMW.RequireAuth(pokerH.CreateTable))
	mux.HandleFunc("POST /api/games/poker/tables/{id}/join", authMW.RequireAuth(pokerH.JoinTable))
	mux.HandleFunc("POST /api/games/poker/tables/{id}/leave", authMW.RequireAuth(pokerH.LeaveTable))
	mux.HandleFunc("POST /api/games/poker/tables/{id}/action", authMW.RequireAuth(pokerH.PlayerAction))
	mux.HandleFunc("POST /api/games/poker/tables/{id}/close", authMW.RequireAuth(pokerH.CloseTable))
	mux.HandleFunc("GET /api/games/poker/tables/{id}", authMW.RequireAuth(pokerH.GetTableState))

	// History & Audit
	mux.HandleFunc("GET /api/history", authMW.RequireAuth(historyH.GetPlayerHistory))

	// Leaderboard & Stats
	mux.HandleFunc("GET /api/stats/player", authMW.RequireAuth(statsH.GetPlayerStats))
	mux.HandleFunc("GET /api/stats/player/{game}", authMW.RequireAuth(statsH.GetPlayerGameStats))

	// Middlewares globales: CORS y Logger
	return middleware.CORS(middleware.Logger(mux))
}
