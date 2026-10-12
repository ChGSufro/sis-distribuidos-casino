## Tareas Lunes 12 (cada uno en una rama nombre-apellido)

### 👤 Chris (Wallet Service y Blackjack Game Service)
#### Wallet Service (`wallet_db` — PostgreSQL)
- [x] Diseñar esquema SQL e inicialización para tablas `wallets`, `holds` y `transactions`.
- [x] Implementar libro contable ACID con restricción `CHECK (balance_cents >= 0)` y control de concurrencia optimista (`version`).
- [x] Implementar flujo transaccional de reservas: `Hold` $\rightarrow$ `Settle` / `Release`.
- [x] Exponer métodos gRPC: `InitWallet`, `GetBalance`, `Deposit`, `Hold`, `Settle`, `Release`.
- [x] Crear `Dockerfile` y configuración de PostgreSQL para Docker Compose.
#### Blackjack Game Service (`bj_state_db` — MongoDB)
- [x] Configurar conexión y persistencia en MongoDB para la colección `hands`.
- [x] Implementar motor de Blackjack: valor de manos con Ases, split, double down, surrender y turno automático de la casa (dealer).
- [x] Implementar cliente gRPC hacia Wallet (`Hold`, `Settle`, `Release`) y History (`RecordGameEvent`).
- [x] Manejo de resiliencia ante caídas de Wallet o History (timeouts y estados cancelados).
- [x] Crear `Dockerfile` y configuración de MongoDB para Docker Compose.

---

### 👤 Yoa (History & Audit Service y Leaderboard & Stats Service)
#### History & Audit Service (`audit_history_db` — MongoDB)
- [ ] Configurar persistencia MongoDB append-only para la colección inmutable `game_events`.
- [ ] Implementar `RecordGameEvent` con validación de idempotencia por `eventId`.
- [ ] Implementar consulta de historial con paginación (`GetPlayerHistory`).
- [ ] Implementar servidor de streaming gRPC saliente `StreamGameEvents` para alimentar estadísticas.
- [ ] Crear `Dockerfile` y configuración de MongoDB para Docker Compose.
#### Leaderboard & Stats Service (`stats_db` — MongoDB)
- [ ] Configurar persistencia MongoDB para colecciones `player_stats`, `player_game_stats` y `rankings`.
- [ ] Implementar cliente gRPC que consuma el stream de History y calcule métricas incrementales (win rate, rachas, ganancia neta).
- [ ] Implementar métodos gRPC de consulta: `GetPlayerStats`, `GetPlayerGameStats` y `GetLeaderboard`.
- [ ] Resiliencia ante desconexiones del stream (reconectar y reintentar periódicamente sin caer).
- [ ] Crear `Dockerfile` y configuración de MongoDB para Docker Compose.

---

### 👤 Saez (Auth & Users Service y Poker Game Service)
#### Auth & Users Service (`users_db` — PostgreSQL)
- [ ] Diseñar esquema SQL para tablas `users` y `sessions`.
- [ ] Implementar hashing seguro de contraseñas (bcrypt) y gestión de sesiones JWT / Refresh Tokens.
- [ ] Implementar cliente gRPC hacia Wallet para invocar `InitWallet` al registrar usuario, con resiliencia ante fallos.
- [ ] Exponer métodos gRPC: `Register`, `Login`, `ValidateToken`, `RefreshToken`, `Logout`, `GetProfile`.
- [ ] Crear `Dockerfile` y configuración de PostgreSQL para Docker Compose.
#### Poker Game Service (`poker_state_db` — MongoDB)
- [ ] Configurar persistencia MongoDB para colecciones `tables` y `rounds`.
- [ ] Implementar motor de póker (Texas Hold'em): fases preflop a showdown, cálculo de potes principales y secundarios (side-pots).
- [ ] Implementar control de turnos y timeout para auto-fold de jugadores inactivos.
- [ ] Integrar cliente gRPC hacia Wallet (`Hold` para Buy-In, `Settle` para Payout) y History (`RecordGameEvent`).
- [ ] Crear `Dockerfile` y configuración de MongoDB para Docker Compose.

## Tareas Viernes 16

### Integración, Plataforma y Entrega Final
- [ ] API Gateway (HTTP → gRPC)
- [ ] Orquestación con Docker Compose
- [ ] Frontend Integrado
- [ ] Pruebas de Flujos End-to-End
- [ ] Documentación y README Operacional