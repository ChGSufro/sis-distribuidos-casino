# Especificación y Diseño de Arquitectura Distribuida

## Sistema de Casino Online

* Alex Saez
* Christian Gajardo
* Yoandri Villarroel

### 1. Información General

* **Nombre del Sistema:** Sistema de Casino Online
* **Dominio:** Juegos de azar en línea, gestión transaccional de créditos virtuales y auditoría en tiempo real.
* **Contexto Académico:** Sistemas Distribuidos y Escalables.
* **Propósito:** Sistema de casino online que permite a los usuarios participar en distintos juegos utilizando créditos virtuales, gestionar su saldo y consultar el historial de sus actividades. El sistema contempla juegos de casino generales, con especial consideración de juegos de cartas como Blackjack y Póker.

### 2. Diagrama de Arquitectura Distribuida (Mermaid)

```mermaid
flowchart TB
    %% ==========================================
    %% CAPA DE CLIENTES Y ACCESO
    %% ==========================================
    subgraph CapaClientes ["Capa de Clientes"]
        Player(["👤 Jugador (Player)"])
        Frontend["📱 Frontend Web / Mobile<br/><i>(TypeScript / React)</i>"]
        Player -->|HTTPS / WSS| Frontend
    end

    subgraph Perimetro ["Perímetro de Red"]
        Gateway["🛡️ API Gateway (Go / Envoy)<br/>• Reverse Proxy & Auth Validation<br/>• Terminación WebSockets & Rate Limiting"]
    end

    Frontend -->|HTTPS / WSS| Gateway

    %% ==========================================
    %% CAPA DE NEGOCIO Y PERSISTENCIA
    %% ==========================================
    subgraph CapaNegocio ["Capa de Servicios de Negocio y Persistencia (Database-per-Service)"]

        %% Dominio 1: Cuentas y Finanzas (Izquierda)
        subgraph DomFinanzas ["Dominio: Cuentas y Finanzas"]
            direction TB
            AuthSvc["Auth & Users Service (Go)<br/>• Autenticación y Perfiles"]
            DB_Users[("users_db<br/>(PostgreSQL)")]
            AuthSvc -.->|SQL/TCP| DB_Users

            WalletSvc["Wallet Service (Go)<br/>• Ledger ACID y Balances"]
            DB_Wallet[("wallet_db<br/>(PostgreSQL)")]
            WalletSvc -.->|SQL/TCP| DB_Wallet

            AuthSvc -->|gRPC: InitWallet| WalletSvc
        end

        %% Dominio 2: Motores de Juego (Centro)
        subgraph DomJuegos ["Dominio: Motores de Juego"]
            direction TB
            BJGameSvc["Blackjack Game Service (Go)<br/>• Estado de Manos y Reglas BJ"]
            DB_BJ[("bj_state_db<br/>(MongoDB)")]
            BJGameSvc -.->|Mongo Protocol| DB_BJ

            PokerGameSvc["Poker Game Service (Go)<br/>• Mesas, Turnos y Botes"]
            DB_Poker[("poker_state_db<br/>(MongoDB)")]
            PokerGameSvc -.->|Mongo Protocol| DB_Poker
        end

        %% Dominio 3: Auditoría y Estadísticas (Derecha)
        subgraph DomAuditoria ["Dominio: Auditoría y Estadísticas"]
            direction TB
            HistorySvc["History & Audit Service (Go)<br/>• Log Inmutable de Jugadas"]
            DB_History[("audit_history_db<br/>(MongoDB)")]
            HistorySvc -.->|Mongo Protocol| DB_History

            StatsSvc["Leaderboard & Stats Service (Go)<br/>• Rankings y Clasificaciones"]
            DB_Stats[("stats_db<br/>(MongoDB)")]
            StatsSvc -.->|Mongo Protocol| DB_Stats

            HistorySvc -->|gRPC: Stream de Eventos| StatsSvc
        end

    end

    %% ==========================================
    %% CONEXIONES GATEWAY -> SERVICIOS
    %% ==========================================
    Gateway -->|gRPC: Auth / Sesión| AuthSvc
    Gateway -->|gRPC: Operaciones Saldo| WalletSvc
    Gateway -->|gRPC / WS: Blackjack| BJGameSvc
    Gateway -->|gRPC / WS: Poker| PokerGameSvc
    Gateway -->|gRPC: Consultar Historial| HistorySvc
    Gateway -->|gRPC: Consultar Rankings| StatsSvc

    %% ==========================================
    %% COMUNICACIÓN INTER-SERVICIOS
    %% ==========================================
    BJGameSvc -->|gRPC: Hold / Settle Bets| WalletSvc
    PokerGameSvc -->|gRPC: BuyIn / Payout| WalletSvc

    BJGameSvc -->|gRPC: RecordHandAudit| HistorySvc
    PokerGameSvc -->|gRPC: RecordTableAudit| HistorySvc

    %% ==========================================
    %% ESTILOS VISUALES
    %% ==========================================
    classDef client fill:#08427b,stroke:#052e56,stroke-width:2px,color:#fff;
    classDef gateway fill:#0f52ba,stroke:#083170,stroke-width:2px,color:#fff;
    classDef service fill:#2563eb,stroke:#1d4ed8,stroke-width:2px,color:#fff;
    classDef database fill:#0284c7,stroke:#0369a1,stroke-width:2px,color:#fff;

    class Player,Frontend client;
    class Gateway gateway;
    class AuthSvc,WalletSvc,BJGameSvc,PokerGameSvc,HistorySvc,StatsSvc service;
    class DB_Users,DB_Wallet,DB_BJ,DB_Poker,DB_History,DB_Stats database;
```

### 3. Estructura de Bases de Datos (Database-per-Service)

Cada microservicio es dueño exclusivo de su base de datos. Ningún servicio puede
acceder directamente a la base de datos de otro servicio. Las referencias entre
servicios se manejan mediante identificadores externos (UUID) transmitidos por
gRPC, sin _foreign keys_ entre bases de datos.

---

#### 3.1 Auth & Users Service → `users_db` (PostgreSQL)

**Responsabilidad única:** autenticación, registro de usuarios y gestión de
sesiones. No conoce créditos, juegos ni estadísticas. Al registrar un usuario
invoca `Wallet.InitWallet` vía gRPC.

##### Tabla: `users`

| Columna | Tipo | Restricciones | Descripción |
|---|---|---|---|
| `id` | `UUID` | `PRIMARY KEY` | Identificador único del usuario |
| `username` | `VARCHAR(50)` | `UNIQUE NOT NULL` | Nombre de usuario público |
| `email` | `VARCHAR(255)` | `UNIQUE NOT NULL` | Correo electrónico |
| `password_hash` | `VARCHAR(255)` | `NOT NULL` | Hash bcrypt de la contraseña |
| `status` | `VARCHAR(20)` | `NOT NULL DEFAULT 'active'` | Estado del usuario: `active`, `inactive`, `banned` |
| `created_at` | `TIMESTAMPTZ` | `NOT NULL DEFAULT NOW()` | Fecha de registro |
| `updated_at` | `TIMESTAMPTZ` | `NOT NULL DEFAULT NOW()` | Última modificación del perfil |
| `last_login_at` | `TIMESTAMPTZ` | | Última fecha de inicio de sesión |

##### Tabla: `sessions`

| Columna | Tipo | Restricciones | Descripción |
|---|---|---|---|
| `id` | `UUID` | `PRIMARY KEY` | Identificador de la sesión |
| `user_id` | `UUID` | `NOT NULL REFERENCES users(id)` | Usuario al que pertenece la sesión |
| `token_hash` | `VARCHAR(255)` | `NOT NULL` | Hash SHA-256 del access token JWT |
| `refresh_token_hash` | `VARCHAR(255)` | | Hash del refresh token para renovación |
| `expires_at` | `TIMESTAMPTZ` | `NOT NULL` | Fecha de expiración del token |
| `revoked_at` | `TIMESTAMPTZ` | | Fecha de revocación (NULL = activa) |
| `created_at` | `TIMESTAMPTZ` | `NOT NULL DEFAULT NOW()` | Fecha de creación de la sesión |

**Índices adicionales:** `idx_sessions_user_id ON sessions(user_id)`,
`idx_sessions_token_hash ON sessions(token_hash)`.

---

#### 3.2 Wallet Service → `wallet_db` (PostgreSQL)

**Responsabilidad única:** libro contable ACID de créditos virtuales. Soporta
las operaciones `InitWallet`, `Hold` (reservar fondos), `Settle` (liquidar
apuesta) y `Release` (liberar reserva). No conoce reglas de juego ni datos de
usuario más allá del `user_id` externo.

##### Tabla: `wallets`

| Columna | Tipo | Restricciones | Descripción |
|---|---|---|---|
| `id` | `UUID` | `PRIMARY KEY` | Identificador único de la billetera |
| `user_id` | `UUID` | `UNIQUE NOT NULL` | Referencia externa al usuario en Auth Service |
| `balance_cents` | `BIGINT` | `NOT NULL DEFAULT 0 CHECK (balance_cents >= 0)` | Saldo disponible en centavos (nunca negativo) |
| `version` | `INTEGER` | `NOT NULL DEFAULT 1` | Control de concurrencia optimista |
| `created_at` | `TIMESTAMPTZ` | `NOT NULL DEFAULT NOW()` | Fecha de creación de la billetera |
| `updated_at` | `TIMESTAMPTZ` | `NOT NULL DEFAULT NOW()` | Última operación sobre el saldo |

##### Tabla: `holds`

| Columna | Tipo | Restricciones | Descripción |
|---|---|---|---|
| `id` | `UUID` | `PRIMARY KEY` | Identificador único de la reserva |
| `wallet_id` | `UUID` | `NOT NULL REFERENCES wallets(id)` | Billetera dueña de la reserva |
| `amount_cents` | `BIGINT` | `NOT NULL CHECK (amount_cents > 0)` | Monto reservado en centavos |
| `game_type` | `VARCHAR(20)` | `NOT NULL` | Tipo de juego: `blackjack` o `poker` |
| `game_ref_id` | `UUID` | `NOT NULL` | Referencia externa a la mano (BJ) o sesión (Poker) |
| `status` | `VARCHAR(20)` | `NOT NULL DEFAULT 'active'` | `active`, `settled`, `released` |
| `idempotency_key` | `VARCHAR(64)` | `UNIQUE NOT NULL` | Clave de idempotencia enviada por el servicio de juego |
| `created_at` | `TIMESTAMPTZ` | `NOT NULL DEFAULT NOW()` | Fecha de creación de la reserva |
| `resolved_at` | `TIMESTAMPTZ` | | Fecha de liquidación o liberación |

##### Tabla: `transactions`

| Columna | Tipo | Restricciones | Descripción |
|---|---|---|---|
| `id` | `UUID` | `PRIMARY KEY` | Identificador único de la transacción |
| `wallet_id` | `UUID` | `NOT NULL REFERENCES wallets(id)` | Billetera afectada |
| `hold_id` | `UUID` | `REFERENCES holds(id)` | Reserva asociada (si aplica) |
| `type` | `VARCHAR(20)` | `NOT NULL` | Tipo de operación: `init`, `deposit`, `hold`, `settle`, `release`, `buyin`, `payout` |
| `amount_cents` | `BIGINT` | `NOT NULL` | Monto de la operación (positivo = crédito, negativo = débito) |
| `balance_after_cents` | `BIGINT` | `NOT NULL` | Saldo resultante después de la operación |
| `created_at` | `TIMESTAMPTZ` | `NOT NULL DEFAULT NOW()` | Fecha exacta de la transacción |

**Índices adicionales:** `idx_holds_wallet_id ON holds(wallet_id)`,
`idx_holds_idempotency ON holds(idempotency_key)`,
`idx_transactions_wallet_id ON transactions(wallet_id, created_at DESC)`.

**Flujo de apuesta (ejemplo Blackjack):**
1. El Blackjack Service llama `Hold(userId, amount, gameRefId, idempotencyKey)` →
   se crea un `hold` con `status = 'active'` y una `transaction` de tipo `hold`
   que descuenta el saldo.
2. Al finalizar la mano, el Blackjack Service llama
   `Settle(holdId, winAmount)` → el `hold` pasa a `status = 'settled'` y se
   registra una `transaction` de tipo `settle` abonando el monto ganado.
3. Si la mano se cancela, se llama `Release(holdId)` → el `hold` pasa a
   `status = 'released'` y se registra una `transaction` de tipo `release`
   que devuelve los fondos al saldo.

---

#### 3.3 Blackjack Game Service → `bj_state_db` (MongoDB)

**Responsabilidad única:** gestión del estado de manos de Blackjack (cartas,
decisiones del jugador, resultado). No almacena créditos ni datos de usuario
más allá del `userId` externo. Al iniciar y finalizar una mano invoca al Wallet
Service (`Hold`/`Settle`/`Release`) y al History Service (`RecordHandAudit`).

##### Colección: `hands`

```json
{
  "_id": "ObjectId",
  "userId": "UUID (referencia externa al Auth Service)",
  "subHands": [
    {
      "index": "Number (0 = mano principal, 1+ = resultado de split)",
      "cards": [
        { "suit": "H | D | C | S", "rank": "2..10 | J | Q | K | A", "value": "Number (1..11)" }
      ],
      "betAmount": "Number (centavos apostados en esta sub-mano; se duplica en double down)",
      "holdId": "UUID (hold en Wallet Service asociado a esta sub-mano)",
      "status": "active | stand | bust | blackjack | doubled | settled | cancelled",
      "result": "win | lose | push | blackjack_win | surrender | null (si no ha terminado)"
    }
  ],
  "dealerCards": [
    {
      "suit": "H | D | C | S",
      "rank": "2..10 | J | Q | K | A",
      "value": "Number (1..11)",
      "hidden": "Boolean (true mientras la carta no se revela al jugador; se expone en dealer_turn)"
    }
  ],
  "playerActions": [
    {
      "subHandIndex": "Number (índice de la sub-mano afectada por la acción)",
      "action": "hit | stand | double | split | surrender",
      "timestamp": "ISODate"
    }
  ],
  "status": "active | dealer_turn | settled | cancelled",
  "totalWinAmount": "Number | null (centavos totales ganados sumando todas las sub-manos)",
  "idempotencyKey": "String (clave única para operaciones con Wallet y History)",
  "createdAt": "ISODate",
  "settledAt": "ISODate | null"
}
```

**Índices:**
- `{ userId: 1, createdAt: -1 }` — historial de manos por jugador.
- `{ "subHands.holdId": 1 }` — búsqueda por reserva de Wallet.
- `{ status: 1 }` — filtrado de manos activas.
- `{ idempotencyKey: 1 }` (único) — prevención de duplicados.

**Ciclo de vida de una mano:**
1. El jugador envía `CreateHand(betAmount)` → se crea el documento con una
   sub-mano inicial (`subHands[0]`, `index = 0`), `status = 'active'`. El
   servicio invoca `Wallet.Hold` y almacena el `holdId` en `subHands[0].holdId`.
2. El jugador envía acciones sobre la sub-mano correspondiente:
   - `hit` → se agrega una carta a `subHands[i].cards`.
   - `stand` → `subHands[i].status = 'stand'`.
   - `double` → se invoca un nuevo `Wallet.Hold` por el monto adicional, se
     duplica `subHands[i].betAmount`, se agrega exactamente una carta y la
     sub-mano queda en `stand` o `bust`.
   - `split` → la sub-mano actual se divide: se crea una nueva sub-mano
     (`index = N+1`) con una de las dos cartas del par. Se invoca un nuevo
     `Wallet.Hold` para la nueva sub-mano. Cada sub-mano recibe una carta
     adicional y continúa de forma independiente.
   - `surrender` → `subHands[i].result = 'surrender'`. Se invoca
     `Wallet.Settle(holdId, betAmount/2)` devolviendo la mitad de la apuesta.
3. Cuando todas las sub-manos están resueltas (`stand`, `bust`, `blackjack`
   o `surrender`), el estado general pasa a `status = 'dealer_turn'`. Se
   revelan las cartas ocultas del dealer (`hidden = false`) y el dealer juega
   según las reglas de la casa.
4. Se determina `result` para cada sub-mano comparando con el dealer. El
   servicio invoca `Wallet.Settle` o `Wallet.Release` por cada sub-mano según
   corresponda, calcula `totalWinAmount`, luego invoca
   `History.RecordHandAudit`. Finalmente `status = 'settled'` (o `'cancelled'`).

---

#### 3.4 Poker Game Service → `poker_state_db` (MongoDB)

**Responsabilidad única:** gestión de mesas de Póker (jugadores, posiciones,
cartas comunitarias, pozos, turnos, ciegas). No administra créditos
directamente: invoca al Wallet Service para `BuyIn`/`Payout`. No conoce
historial de auditoría: invoca `History.RecordTableAudit`.

El modelo separa la **mesa persistente** (`tables`) de cada **ronda/mano
individual** (`rounds`). Una mesa puede albergar decenas de rondas sucesivas;
los stacks de los jugadores se mantienen en la mesa entre rondas.

##### Colección: `tables`

Representa la mesa de Póker como entidad persistente. Los jugadores sentados
y sus fichas acumuladas viven aquí y sobreviven entre rondas.

```json
{
  "_id": "ObjectId",
  "name": "String (nombre visible de la mesa, ej. 'Mesa #1')",
  "smallBlind": "Number (centavos, > 0)",
  "bigBlind": "Number (centavos, > 0)",
  "maxPlayers": "Number (2..10)",
  "status": "waiting | active | closed",
  "seats": [
    {
      "seatPosition": "Number (0..maxPlayers-1)",
      "userId": "UUID (referencia externa al Auth Service)",
      "stack": "Number (fichas disponibles en mesa, centavos)",
      "holdId": "UUID (referencia al hold del buy-in en Wallet Service)",
      "status": "active | sitting_out | left"
    }
  ],
  "currentRoundId": "ObjectId | null (referencia a la ronda en curso en rounds)",
  "createdAt": "ISODate"
}
```

##### Colección: `rounds`

Representa una mano individual de Póker dentro de una mesa. Se crea una nueva
por cada mano y contiene las cartas privadas de cada jugador, las cartas
comunitarias, los botes, las acciones y los ganadores.

```json
{
  "_id": "ObjectId",
  "tableId": "ObjectId (referencia a tables._id)",
  "roundNumber": "Number (número secuencial de la mano dentro de la mesa)",
  "players": [
    {
      "userId": "UUID (referencia externa al Auth Service)",
      "seatPosition": "Number (0..maxPlayers-1)",
      "holeCards": [
        { "suit": "H | D | C | S", "rank": "2..10 | J | Q | K | A" }
      ],
      "stackAtStart": "Number (fichas del jugador al inicio de la ronda)",
      "totalBetInRound": "Number (total apostado por este jugador en la ronda)",
      "status": "active | folded | all-in | eliminated"
    }
  ],
  "communityCards": [
    { "suit": "H | D | C | S", "rank": "2..10 | J | Q | K | A" }
  ],
  "potSize": "Number (centavos acumulados en el bote principal)",
  "sidePots": [
    {
      "amount": "Number (centavos del bote secundario)",
      "eligiblePlayers": ["UUID (userId de jugadores elegibles)"]
    }
  ],
  "dealerPosition": "Number (seatPosition del dealer)",
  "currentTurn": "Number (seatPosition del jugador que debe actuar)",
  "currentBet": "Number (apuesta actual a igualar en la ronda de apuestas vigente)",
  "turnExpiresAt": "ISODate (tiempo límite para que el jugador actual actúe; auto-fold al expirar)",
  "stage": "preflop | flop | turn | river | showdown | settled",
  "actions": [
    {
      "userId": "UUID",
      "action": "fold | check | call | raise | all-in",
      "amount": "Number (centavos apostados, 0 para fold/check)",
      "timestamp": "ISODate"
    }
  ],
  "winners": [
    {
      "userId": "UUID",
      "potType": "main | side",
      "amount": "Number (centavos ganados)"
    }
  ],
  "createdAt": "ISODate",
  "settledAt": "ISODate | null"
}
```

**Índices `tables`:**
- `{ "seats.userId": 1 }` — búsqueda de mesas donde un jugador está sentado.
- `{ status: 1 }` — filtrado de mesas activas, en espera o cerradas.

**Índices `rounds`:**
- `{ tableId: 1, roundNumber: -1 }` — rondas por mesa en orden descendente.
- `{ "players.userId": 1 }` — búsqueda de rondas en las que participó un jugador.
- `{ stage: 1 }` — rondas en curso vs. finalizadas.

**Ciclo de vida de una mesa y sus rondas:**
1. Un jugador crea o se une a una mesa. Al sentarse invoca `Wallet.Hold` como
   buy-in; el `holdId` se almacena en `seats[].holdId` de la mesa y las fichas
   se acreditan en `seats[].stack`.
2. Cuando hay suficientes jugadores activos, se crea un documento en `rounds`
   con `roundNumber` secuencial. Se reparten `holeCards` a cada jugador,
   `stage = 'preflop'`. La mesa actualiza `currentRoundId`.
3. La ronda avanza por `preflop → flop → turn → river → showdown`. En cada
   etapa se registran las acciones en `actions[]`, se actualizan `potSize`,
   `sidePots`, `currentBet` y `currentTurn`. Si `turnExpiresAt` expira sin
   acción, el sistema ejecuta auto-fold para el jugador.
4. En `showdown` se evalúan las mejores manos combinando `holeCards` +
   `communityCards`. Los ganadores se registran en `winners[]`. Los stacks
   en `tables.seats[]` se actualizan (ganadores aumentan, perdedores
   disminuyen). `stage → 'settled'`, `settledAt = NOW()`.
5. Al finalizar la ronda, el servicio invoca `History.RecordTableAudit` con
   el resumen. Se puede iniciar una nueva ronda repitiendo desde el paso 2.
6. Cuando un jugador abandona la mesa, el servicio invoca
   `Wallet.Settle(holdId, remainingStack)` para devolver su stack restante
   a su billetera. Si el stack es 0, se liquida con `Settle(holdId, 0)`.

---

#### 3.5 History & Audit Service → `audit_history_db` (MongoDB)

**Responsabilidad única:** registro inmutable de eventos de juego. Solo
escritura (_append-only_). Recibe eventos de los servicios de juego
(`RecordHandAudit` y `RecordTableAudit`) y transmite un stream de eventos al
Stats Service vía gRPC.

##### Colección: `game_events`

```json
{
  "_id": "ObjectId",
  "eventId": "UUID (clave de idempotencia, única)",
  "userId": "UUID (referencia externa al Auth Service)",
  "gameType": "blackjack | poker",
  "gameRefId": "String (hand._id en BJ o rounds._id en Poker)",
  "action": "hand_result | bet_placed | hand_cancelled | table_result | player_buyin | player_payout | player_eliminated",
  "details": {
    "(varía según gameType y action, ver ejemplos abajo)": "..."
  },
  "timestamp": "ISODate"
}
```

**Estructura de `details` según acción y tipo de juego:**

`hand_result` (Blackjack):
```json
{
  "betAmount": 1000,
  "result": "win",
  "winAmount": 2000,
  "playerCards": [{ "suit": "H", "rank": "A" }, { "suit": "S", "rank": "K" }],
  "dealerCards": [{ "suit": "D", "rank": "7" }, { "suit": "C", "rank": "Q" }]
}
```

`bet_placed` (Blackjack):
```json
{
  "betAmount": 500,
  "handId": "UUID de la mano"
}
```

`hand_cancelled` (Blackjack):
```json
{
  "reason": "player_disconnected | timeout",
  "betAmount": 1000
}
```

`table_result` (Poker):
```json
{
  "finalStage": "showdown",
  "yourResult": "win",
  "winAmount": 3500,
  "communityCards": [{ "suit": "H", "rank": "A" }, { "suit": "H", "rank": "K" }, { "suit": "H", "rank": "Q" }, { "suit": "H", "rank": "J" }, { "suit": "H", "rank": "10" }],
  "yourHand": [{ "suit": "S", "rank": "A" }, { "suit": "D", "rank": "A" }],
  "playerCount": 5,
  "potSize": 12000
}
```

`player_buyin` / `player_payout` (Poker):
```json
{
  "tableId": "ObjectId de la mesa",
  "amount": 5000,
  "holdId": "UUID del hold en Wallet"
}
```

**Índices:**
- `{ userId: 1, timestamp: -1 }` — historial de un jugador ordenado por fecha.
- `{ gameType: 1, timestamp: -1 }` — eventos filtrados por tipo de juego.
- `{ eventId: 1 }` (único) — prevención de duplicados.

**Principio de inmutabilidad:** los documentos en `game_events` nunca se
modifican ni eliminan. Cualquier corrección se registra como un nuevo evento
con un `action` de tipo compensación.

---

#### 3.6 Leaderboard & Stats Service → `stats_db` (MongoDB)

**Responsabilidad única:** rankings agregados y series temporales de
estadísticas por jugador. Recibe un stream de eventos desde el History Service
y actualiza agregados incrementalmente. No almacena eventos crudos ni detalles
de manos o mesas.

##### Colección: `player_stats`

```json
{
  "_id": "ObjectId",
  "userId": "UUID (referencia externa al Auth Service)",
  "totalWagered": "Number (centavos totales apostados)",
  "totalWon": "Number (centavos totales ganados)",
  "netProfit": "Number (totalWon - totalWagered, calculado)",
  "gamesPlayed": "Number (total de manos o sesiones completadas)",
  "winRate": "Number (proporción de victorias, 0.0..1.0)",
  "largestWin": "Number (mayor ganancia individual en centavos)",
  "currentStreak": "Number (racha actual: positivo = victorias, negativo = derrotas)",
  "bestStreak": "Number (mejor racha histórica de victorias)",
  "lastUpdated": "ISODate"
}
```

##### Colección: `player_game_stats`

```json
{
  "_id": "ObjectId",
  "userId": "UUID",
  "gameType": "blackjack | poker",
  "gamesPlayed": "Number",
  "totalWagered": "Number (centavos)",
  "totalWon": "Number (centavos)",
  "largestWin": "Number (centavos)",
  "lastUpdated": "ISODate"
}
```

##### Colección: `rankings`

```json
{
  "_id": "ObjectId",
  "period": "daily | weekly | monthly | all_time",
  "periodStart": "ISODate (inicio del período)",
  "entries": [
    {
      "userId": "UUID",
      "rank": "Number (posición 1..N)",
      "score": "Number (netProfit del período)",
      "gamesPlayed": "Number"
    }
  ],
  "computedAt": "ISODate (fecha de cálculo del ranking)"
}
```

**Índices:**
- `{ userId: 1 }` (único en `player_stats`) — estadísticas por jugador.
- `{ userId: 1, gameType: 1 }` (único compuesto en `player_game_stats`) — estadísticas por jugador y tipo de juego.
- `{ period: 1, periodStart: -1 }` en `rankings` — consulta del ranking más reciente por período.

**Mecanismo de actualización:**
1. El History Service emite eventos por streaming gRPC al Stats Service.
2. Stats Service actualiza incrementalmente `player_stats` y
   `player_game_stats` para cada evento recibido (nunca consulta
   `audit_history_db` directamente).
3. Periódicamente (cada hora/día/semana/mes) se calculan snapshots de
   `rankings` ordenando por `netProfit` descendente.

---

### 4. Resumen de Bases de Datos por Servicio

| Servicio | Base de Datos | Motor | Recurso principal |
|---|---|---|---|
| Auth & Users | `users_db` | PostgreSQL | Tablas: `users`, `sessions` |
| Wallet | `wallet_db` | PostgreSQL | Tablas: `wallets`, `holds`, `transactions` |
| Blackjack Game | `bj_state_db` | MongoDB | Colección: `hands` |
| Poker Game | `poker_state_db` | MongoDB | Colecciones: `tables`, `rounds` |
| History & Audit | `audit_history_db` | MongoDB | Colección: `game_events` |
| Leaderboard & Stats | `stats_db` | MongoDB | Colecciones: `player_stats`, `player_game_stats`, `rankings` |
