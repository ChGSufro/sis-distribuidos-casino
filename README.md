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
| `game_ref_id` | `VARCHAR(64)` | `NOT NULL` | Referencia externa al juego, como ObjectId hex: `hands._id` en Blackjack, `tables._id` en Póker |
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
   `Settle(holdId, winAmountCents)` → el `hold` pasa a `status = 'settled'` y se
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
      "betAmountCents": "Number (centavos apostados en esta sub-mano; se duplica en double down)",
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
  "totalWinAmountCents": "Number | null (centavos totales ganados sumando todas las sub-manos)",
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
1. El jugador envía `CreateHand(betAmountCents)` → se crea el documento con una
   sub-mano inicial (`subHands[0]`, `index = 0`), `status = 'active'`. El
   servicio invoca `Wallet.Hold` y almacena el `holdId` en `subHands[0].holdId`.
2. El jugador envía acciones sobre la sub-mano correspondiente:
   - `hit` → se agrega una carta a `subHands[i].cards`.
   - `stand` → `subHands[i].status = 'stand'`.
   - `double` → se invoca un nuevo `Wallet.Hold` por el monto adicional, se
     duplica `subHands[i].betAmountCents`, se agrega exactamente una carta y la
     sub-mano queda en `stand` o `bust`.
   - `split` → la sub-mano actual se divide: se crea una nueva sub-mano
     (`index = N+1`) con una de las dos cartas del par. Se invoca un nuevo
     `Wallet.Hold` para la nueva sub-mano. Cada sub-mano recibe una carta
     adicional y continúa de forma independiente.
   - `surrender` → `subHands[i].result = 'surrender'`. Se invoca
     `Wallet.Settle(holdId, betAmountCents/2)` devolviendo la mitad de la apuesta.
3. Cuando todas las sub-manos están resueltas (`stand`, `bust`, `blackjack`
   o `surrender`), el estado general pasa a `status = 'dealer_turn'`. Se
   revelan las cartas ocultas del dealer (`hidden = false`) y el dealer juega
   según las reglas de la casa.
4. Se determina `result` para cada sub-mano comparando con el dealer. El
   servicio invoca `Wallet.Settle` o `Wallet.Release` por cada sub-mano según
   corresponda, calcula `totalWinAmountCents`, luego invoca
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
  "smallBlindCents": "Number (centavos, > 0)",
  "bigBlindCents": "Number (centavos, > 0)",
  "maxPlayers": "Number (2..10)",
  "status": "waiting | active | closed",
  "seats": [
    {
      "seatPosition": "Number (0..maxPlayers-1)",
      "userId": "UUID (referencia externa al Auth Service)",
      "stackCents": "Number (fichas disponibles en mesa, centavos)",
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
      "stackAtStartCents": "Number (fichas del jugador al inicio de la ronda)",
      "totalBetInRoundCents": "Number (total apostado por este jugador en la ronda)",
      "status": "active | folded | all-in | eliminated"
    }
  ],
  "communityCards": [
    { "suit": "H | D | C | S", "rank": "2..10 | J | Q | K | A" }
  ],
  "potSizeCents": "Number (centavos acumulados en el bote principal)",
  "sidePots": [
    {
      "amountCents": "Number (centavos del bote secundario)",
      "eligiblePlayers": ["UUID (userId de jugadores elegibles)"]
    }
  ],
  "dealerPosition": "Number (seatPosition del dealer)",
  "currentTurn": "Number (seatPosition del jugador que debe actuar)",
  "currentBetCents": "Number (apuesta actual a igualar en la ronda de apuestas vigente)",
  "turnExpiresAt": "ISODate (tiempo límite para que el jugador actual actúe; auto-fold al expirar)",
  "stage": "preflop | flop | turn | river | showdown | settled",
  "actions": [
    {
      "userId": "UUID",
      "action": "fold | check | call | raise | all-in",
      "amountCents": "Number (centavos apostados, 0 para fold/check)",
      "timestamp": "ISODate"
    }
  ],
  "winners": [
    {
      "userId": "UUID",
      "potType": "main | side",
      "amountCents": "Number (centavos ganados)"
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
   se acreditan en `seats[].stackCents`.
2. Cuando hay suficientes jugadores activos, se crea un documento en `rounds`
   con `roundNumber` secuencial. Se reparten `holeCards` a cada jugador,
   `stage = 'preflop'`. La mesa actualiza `currentRoundId`.
3. La ronda avanza por `preflop → flop → turn → river → showdown`. En cada
etapa se registran las acciones en `actions[]`, se actualizan `potSizeCents`,
    `sidePots`, `currentBetCents` y `currentTurn`. Si `turnExpiresAt` expira sin
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
  "betAmountCents": 1000,
  "result": "win",
  "winAmountCents": 2000,
  "playerCards": [{ "suit": "H", "rank": "A" }, { "suit": "S", "rank": "K" }],
  "dealerCards": [{ "suit": "D", "rank": "7" }, { "suit": "C", "rank": "Q" }]
}
```

`bet_placed` (Blackjack):
```json
{
  "betAmountCents": 500,
  "handId": "UUID de la mano"
}
```

`hand_cancelled` (Blackjack):
```json
{
  "reason": "player_disconnected | timeout",
  "betAmountCents": 1000
}
```

`table_result` (Poker):
```json
{
  "finalStage": "showdown",
  "yourResult": "win",
  "winAmountCents": 3500,
  "communityCards": [{ "suit": "H", "rank": "A" }, { "suit": "H", "rank": "K" }, { "suit": "H", "rank": "Q" }, { "suit": "H", "rank": "J" }, { "suit": "H", "rank": "10" }],
  "yourHand": [{ "suit": "S", "rank": "A" }, { "suit": "D", "rank": "A" }],
  "playerCount": 5,
  "potSizeCents": 12000
}
```

`player_buyin` / `player_payout` (Poker):
```json
{
  "tableId": "ObjectId de la mesa",
  "amountCents": 5000,
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
  "totalWageredCents": "Number (centavos totales apostados)",
  "totalWonCents": "Number (centavos totales ganados)",
  "netProfitCents": "Number (totalWonCents - totalWageredCents, calculado)",
  "gamesPlayed": "Number (total de manos o sesiones completadas)",
  "winRate": "Number (proporción de victorias, 0.0..1.0)",
  "largestWinCents": "Number (mayor ganancia individual en centavos)",
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
  "totalWageredCents": "Number (centavos)",
  "totalWonCents": "Number (centavos)",
  "largestWinCents": "Number (centavos)",
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
      "scoreCents": "Number (netProfit del período)",
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
   `rankings` ordenando por `netProfitCents` descendente.

---

### 4. Contratos entre Servicios (Interfaces y DTOs)

Esta sección define **qué datos se envían y reciben** entre cada par de
servicios. Cada operación muestra su solicitud y respuesta como objetos DTO
(Data Transfer Object) con el tipo y propósito de cada campo.

> **Convención:** todos los montos monetarios están en **centavos** (enteros)
> para evitar errores de punto flotante. Los identificadores de usuario son
> siempre el `UUID` emitido por el Auth Service.

---

#### 4.1 Auth & Users Service

Operaciones que este servicio expone. Es llamado por el **API Gateway**
(acciones del usuario) y llama al **Wallet Service** al registrar un usuario.

##### `Register`

**Flujo:** Frontend → Gateway → Auth Service → _(internamente llama `InitWallet` al Wallet Service)_

**Solicitud:**
```json
{
  "username": "String — nombre de usuario deseado",
  "email": "String — correo electrónico",
  "password": "String — contraseña en texto plano (se hashea en el servicio)"
}
```

**Respuesta:**
```json
{
  "userId": "UUID — identificador del usuario creado",
  "username": "String",
  "email": "String",
  "createdAt": "ISODate"
}
```

##### `Login`

**Flujo:** Frontend → Gateway → Auth Service

**Solicitud:**
```json
{
  "email": "String — correo electrónico",
  "password": "String — contraseña en texto plano"
}
```

**Respuesta:**
```json
{
  "accessToken": "String — JWT firmado para autenticar solicitudes",
  "refreshToken": "String — token para renovar el accessToken sin re-login",
  "expiresAt": "ISODate — fecha de expiración del accessToken",
  "user": {
    "userId": "UUID",
    "username": "String",
    "status": "String — active | inactive | banned"
  }
}
```

##### `ValidateToken`

**Flujo:** Gateway → Auth Service _(en cada solicitud entrante para validar el JWT)_

**Solicitud:**
```json
{
  "accessToken": "String — JWT enviado por el frontend"
}
```

**Respuesta:**
```json
{
  "valid": "Boolean — true si el token es válido y no está revocado",
  "userId": "UUID — identificador del usuario autenticado",
  "username": "String"
}
```

##### `RefreshToken`

**Flujo:** Frontend → Gateway → Auth Service

**Solicitud:**
```json
{
  "refreshToken": "String — refresh token vigente"
}
```

**Respuesta:**
```json
{
  "accessToken": "String — nuevo JWT",
  "refreshToken": "String — nuevo refresh token (rotación)",
  "expiresAt": "ISODate"
}
```

##### `Logout`

**Flujo:** Frontend → Gateway → Auth Service

**Solicitud:**
```json
{
  "accessToken": "String — token a revocar"
}
```

**Respuesta:**
```json
{
  "success": "Boolean"
}
```

##### `GetProfile`

**Flujo:** Frontend → Gateway → Auth Service

**Solicitud:**
```json
{
  "userId": "UUID"
}
```

**Respuesta:**
```json
{
  "userId": "UUID",
  "username": "String",
  "email": "String",
  "status": "String — active | inactive | banned",
  "createdAt": "ISODate",
  "lastLoginAt": "ISODate | null"
}
```

---

#### 4.2 Wallet Service

Operaciones para gestionar créditos virtuales. Es llamado por el **Gateway**
(consultas del usuario), por el **Auth Service** (al registrar), y por los
**servicios de juego** (para reservar y liquidar apuestas).

##### `InitWallet`

**Flujo:** Auth Service → Wallet Service _(al registrar un nuevo usuario)_

**Solicitud:**
```json
{
  "userId": "UUID — usuario para el cual crear la billetera"
}
```

**Respuesta:**
```json
{
  "walletId": "UUID",
  "balanceCents": "Number — saldo inicial (0)"
}
```

##### `GetBalance`

**Flujo:** Frontend → Gateway → Wallet Service

**Solicitud:**
```json
{
  "userId": "UUID"
}
```

**Respuesta:**
```json
{
  "userId": "UUID",
  "balanceCents": "Number — saldo disponible en centavos",
  "updatedAt": "ISODate"
}
```

##### `Deposit`

**Flujo:** Frontend → Gateway → Wallet Service

**Solicitud:**
```json
{
  "userId": "UUID",
  "amountCents": "Number — monto a depositar (> 0)"
}
```

**Respuesta:**
```json
{
  "transactionId": "UUID — identificador de la transacción de depósito",
  "newBalanceCents": "Number — saldo después del depósito"
}
```

##### `Hold`

**Flujo:** Blackjack / Poker Service → Wallet Service _(reservar fondos antes de jugar)_

**Solicitud:**
```json
{
  "userId": "UUID",
  "amountCents": "Number — monto a reservar (> 0)",
  "gameType": "String — 'blackjack' | 'poker'",
  "gameRefId": "UUID — referencia a la mano (BJ) o mesa (Poker)",
  "idempotencyKey": "String — clave única para evitar reservas duplicadas"
}
```

**Respuesta:**
```json
{
  "holdId": "UUID — identificador de la reserva creada",
  "amountCents": "Number — monto efectivamente reservado",
  "status": "String — 'active'"
}
```

##### `Settle`

**Flujo:** Blackjack / Poker Service → Wallet Service _(liquidar una reserva, devolviendo ganancias)_

**Solicitud:**
```json
{
  "holdId": "UUID — reserva a liquidar",
  "winAmountCents": "Number — monto ganado a abonar (0 si perdió todo)"
}
```

**Respuesta:**
```json
{
  "holdId": "UUID",
  "status": "String — 'settled'",
  "transactionId": "UUID",
  "newBalanceCents": "Number — saldo después de la liquidación"
}
```

##### `Release`

**Flujo:** Blackjack Service → Wallet Service _(cancelar una reserva y devolver fondos completos)_

**Solicitud:**
```json
{
  "holdId": "UUID — reserva a liberar"
}
```

**Respuesta:**
```json
{
  "holdId": "UUID",
  "status": "String — 'released'",
  "transactionId": "UUID",
  "newBalanceCents": "Number — saldo después de liberar la reserva"
}
```

---

#### 4.3 Blackjack Game Service

Operaciones para jugar Blackjack. Es llamado por el **Gateway** (acciones del
jugador vía WebSocket) y llama internamente al **Wallet Service** y al
**History Service**.

##### `CreateHand`

**Flujo:** Frontend → Gateway → Blackjack Service → _(internamente llama `Wallet.Hold`)_

**Solicitud:**
```json
{
  "userId": "UUID",
  "betAmountCents": "Number — apuesta inicial en centavos (> 0)"
}
```

**Respuesta:**
```json
{
  "handId": "String — identificador de la mano creada",
  "subHands": [
    {
      "index": 0,
      "cards": [{ "suit": "String", "rank": "String", "value": "Number" }],
      "betAmountCents": "Number",
      "status": "String — 'active'"
    }
  ],
  "dealerVisibleCard": {
    "suit": "String",
    "rank": "String",
    "value": "Number"
  },
  "status": "String — 'active'"
}
```

> **Nota:** solo se devuelve la carta visible del dealer. La carta oculta
> permanece con `hidden = true` en la base de datos y no se incluye en la
> respuesta hasta que el estado general pase a `dealer_turn`.

##### `PlayerAction`

**Flujo:** Frontend → Gateway → Blackjack Service → _(puede llamar `Wallet.Hold` en double/split)_

**Solicitud:**
```json
{
  "handId": "String",
  "userId": "UUID",
  "subHandIndex": "Number — índice de la sub-mano sobre la que actúa",
  "action": "String — 'hit' | 'stand' | 'double' | 'split' | 'surrender'"
}
```

**Respuesta:**
```json
{
  "handId": "String",
  "subHands": [
    {
      "index": "Number",
      "cards": [{ "suit": "String", "rank": "String", "value": "Number" }],
      "betAmountCents": "Number",
      "status": "String",
      "result": "String | null — solo cuando la sub-mano está resuelta"
    }
  ],
  "dealerCards": [
    {
      "suit": "String",
      "rank": "String",
      "value": "Number",
      "hidden": "Boolean — false si ya fue revelada"
    }
  ],
  "status": "String — estado general de la mano",
  "totalWinAmountCents": "Number | null — solo cuando status = 'settled'"
}
```

##### `GetHandState`

**Flujo:** Frontend → Gateway → Blackjack Service

**Solicitud:**
```json
{
  "handId": "String",
  "userId": "UUID"
}
```

**Respuesta:** misma estructura que la respuesta de `PlayerAction`.

---

#### 4.4 Poker Game Service

Operaciones para gestionar mesas y jugar Póker. Es llamado por el **Gateway**
(acciones del jugador vía HTTP y WebSocket) y llama internamente al
**Wallet Service** y al **History Service**.

##### `ListTables`

**Flujo:** Frontend → Gateway → Poker Service

**Solicitud:**
```json
{
  "statusFilter": "String | null — 'waiting' | 'active' | null (todas)"
}
```

**Respuesta:**
```json
{
  "tables": [
    {
      "tableId": "String",
      "name": "String",
      "smallBlindCents": "Number",
      "bigBlindCents": "Number",
      "maxPlayers": "Number",
      "currentPlayers": "Number — jugadores sentados actualmente",
      "status": "String"
    }
  ]
}
```

##### `CreateTable`

**Flujo:** Frontend → Gateway → Poker Service

**Solicitud:**
```json
{
  "userId": "UUID — usuario que crea la mesa",
  "name": "String — nombre visible de la mesa",
  "smallBlindCents": "Number — ciega pequeña en centavos",
  "bigBlindCents": "Number — ciega grande en centavos",
  "maxPlayers": "Number — máximo de jugadores (2..10)"
}
```

**Respuesta:**
```json
{
  "tableId": "String",
  "name": "String",
  "smallBlindCents": "Number",
  "bigBlindCents": "Number",
  "maxPlayers": "Number",
  "status": "String — 'waiting'"
}
```

##### `JoinTable`

**Flujo:** Frontend → Gateway → Poker Service → _(internamente llama `Wallet.Hold` como buy-in)_

**Solicitud:**
```json
{
  "tableId": "String",
  "userId": "UUID",
  "buyInAmountCents": "Number — cantidad de fichas para comprar (> 0)",
  "preferredSeat": "Number | null — asiento deseado (opcional, se asigna uno libre si no se indica)"
}
```

**Respuesta:**
```json
{
  "tableId": "String",
  "seatPosition": "Number — asiento asignado",
  "stackCents": "Number — fichas acreditadas en mesa",
  "holdId": "UUID — referencia a la reserva del buy-in en Wallet"
}
```

##### `LeaveTable`

**Flujo:** Frontend → Gateway → Poker Service → _(internamente llama `Wallet.Settle`)_

**Solicitud:**
```json
{
  "tableId": "String",
  "userId": "UUID"
}
```

**Respuesta:**
```json
{
  "tableId": "String",
  "finalStackCents": "Number — fichas que tenía al momento de salir",
  "settledAmountCents": "Number — monto devuelto a la billetera"
}
```

##### `PlayerAction`

**Flujo:** Frontend → Gateway (WebSocket) → Poker Service

**Solicitud:**
```json
{
  "tableId": "String",
  "roundId": "String",
  "userId": "UUID",
  "action": "String — 'fold' | 'check' | 'call' | 'raise' | 'all-in'",
  "amountCents": "Number | null — monto del raise (requerido solo para 'raise')"
}
```

**Respuesta (se envía a todos los jugadores de la mesa vía WebSocket):**
```json
{
  "roundId": "String",
  "stage": "String — etapa actual de la ronda",
  "currentTurn": "Number — seatPosition del siguiente jugador que debe actuar",
  "currentBetCents": "Number — apuesta actual a igualar",
  "potSizeCents": "Number — bote total acumulado",
  "communityCards": [{ "suit": "String", "rank": "String" }],
  "players": [
    {
      "userId": "UUID",
      "seatPosition": "Number",
      "stackCents": "Number",
      "totalBetInRoundCents": "Number",
      "status": "String"
    }
  ],
  "turnExpiresAt": "ISODate — tiempo límite para actuar",
  "winners": "Array | null — solo cuando stage = 'settled' (ver estructura abajo)"
}
```

**Estructura de `winners` (cuando la ronda finaliza):**
```json
[
  {
    "userId": "UUID",
    "potType": "String — 'main' | 'side'",
    "amountCents": "Number — centavos ganados"
  }
]
```

##### `GetTableState`

**Flujo:** Frontend → Gateway → Poker Service

**Solicitud:**
```json
{
  "tableId": "String",
  "userId": "UUID — se usa para determinar qué cartas privadas incluir"
}
```

**Respuesta:**
```json
{
  "tableId": "String",
  "name": "String",
  "smallBlindCents": "Number",
  "bigBlindCents": "Number",
  "status": "String",
  "seats": [
    {
      "seatPosition": "Number",
      "userId": "UUID",
      "stackCents": "Number",
      "status": "String"
    }
  ],
  "currentRound": {
    "roundId": "String",
    "stage": "String",
    "communityCards": [{ "suit": "String", "rank": "String" }],
    "yourHoleCards": [{ "suit": "String", "rank": "String" }],
    "potSizeCents": "Number",
    "currentBetCents": "Number",
    "currentTurn": "Number",
    "turnExpiresAt": "ISODate"
  }
}
```

> **Nota:** `yourHoleCards` contiene únicamente las cartas privadas del
> usuario que realiza la consulta. Las cartas de los demás jugadores nunca
> se envían hasta el `showdown`.

---

#### 4.5 History & Audit Service

Operaciones para registrar y consultar eventos de juego. Recibe escrituras
de los servicios de juego y expone lecturas al Gateway. Emite un stream de
eventos al Stats Service.

##### `RecordGameEvent`

**Flujo:** Blackjack / Poker Service → History Service _(al finalizar una mano o acción relevante)_

**Solicitud:**
```json
{
  "eventId": "UUID — clave de idempotencia para evitar duplicados",
  "userId": "UUID",
  "gameType": "String — 'blackjack' | 'poker'",
  "gameRefId": "String — hand._id (BJ) o rounds._id (Poker)",
  "action": "String — 'hand_result' | 'bet_placed' | 'hand_cancelled' | 'table_result' | 'player_buyin' | 'player_payout' | 'player_eliminated'",
  "details": "Object — contenido variable según gameType y action (ver sección 3.5 para ejemplos)",
  "timestamp": "ISODate"
}
```

**Respuesta:**
```json
{
  "recorded": "Boolean — true si se registró correctamente",
  "eventId": "UUID — confirmación del evento registrado"
}
```

##### `GetPlayerHistory`

**Flujo:** Frontend → Gateway → History Service

**Solicitud:**
```json
{
  "userId": "UUID",
  "gameType": "String | null — filtrar por 'blackjack' o 'poker' (null = todos)",
  "limit": "Number — cantidad máxima de eventos a devolver",
  "offset": "Number — desplazamiento para paginación"
}
```

**Respuesta:**
```json
{
  "events": [
    {
      "eventId": "UUID",
      "gameType": "String",
      "gameRefId": "String",
      "action": "String",
      "details": "Object",
      "timestamp": "ISODate"
    }
  ],
  "totalCount": "Number — total de eventos que coinciden con los filtros"
}
```

---

#### 4.6 Leaderboard & Stats Service

Operaciones para consultar estadísticas y rankings. Recibe eventos del
History Service mediante streaming continuo y expone datos agregados al
Gateway.

##### `StreamGameEvents` _(streaming continuo)_

**Flujo:** History Service → Stats Service _(stream unidireccional: el History emite y el Stats consume)_

**Cada evento del stream:**
```json
{
  "eventId": "UUID",
  "userId": "UUID",
  "gameType": "String",
  "action": "String",
  "details": "Object",
  "timestamp": "ISODate"
}
```

El Stats Service procesa cada evento y actualiza incrementalmente
`player_stats` y `player_game_stats`. No envía respuesta por evento
individual (es un flujo de una sola dirección).

##### `GetPlayerStats`

**Flujo:** Frontend → Gateway → Stats Service

**Solicitud:**
```json
{
  "userId": "UUID"
}
```

**Respuesta:**
```json
{
  "userId": "UUID",
  "totalWageredCents": "Number",
  "totalWonCents": "Number",
  "netProfitCents": "Number",
  "gamesPlayed": "Number",
  "winRate": "Number — proporción 0.0 a 1.0",
  "largestWinCents": "Number",
  "currentStreak": "Number — positivo = victorias, negativo = derrotas",
  "bestStreak": "Number",
  "lastUpdated": "ISODate"
}
```

##### `GetPlayerGameStats`

**Flujo:** Frontend → Gateway → Stats Service

**Solicitud:**
```json
{
  "userId": "UUID",
  "gameType": "String — 'blackjack' | 'poker'"
}
```

**Respuesta:**
```json
{
  "userId": "UUID",
  "gameType": "String",
  "gamesPlayed": "Number",
  "totalWageredCents": "Number",
  "totalWonCents": "Number",
  "largestWinCents": "Number",
  "lastUpdated": "ISODate"
}
```

##### `GetLeaderboard`

**Flujo:** Frontend → Gateway → Stats Service

**Solicitud:**
```json
{
  "period": "String — 'daily' | 'weekly' | 'monthly' | 'all_time'",
  "limit": "Number — cantidad de posiciones a devolver (ej. top 10)"
}
```

**Respuesta:**
```json
{
  "period": "String",
  "periodStart": "ISODate",
  "entries": [
    {
      "rank": "Number",
      "userId": "UUID",
      "username": "String",
      "scoreCents": "Number — netProfit del período",
      "gamesPlayed": "Number"
    }
  ],
  "computedAt": "ISODate"
}
```

---

#### 4.7 Mapa de Dependencias entre Servicios

Resumen de quién llama a quién y con qué operaciones:

| Origen | Destino | Operaciones |
|---|---|---|
| **API Gateway** | Auth & Users | `Register`, `Login`, `ValidateToken`, `RefreshToken`, `Logout`, `GetProfile` |
| **API Gateway** | Wallet | `GetBalance`, `Deposit` |
| **API Gateway** | Blackjack Game | `CreateHand`, `PlayerAction`, `GetHandState` |
| **API Gateway** | Poker Game | `ListTables`, `CreateTable`, `JoinTable`, `LeaveTable`, `PlayerAction`, `GetTableState` |
| **API Gateway** | History & Audit | `GetPlayerHistory` |
| **API Gateway** | Leaderboard & Stats | `GetPlayerStats`, `GetPlayerGameStats`, `GetLeaderboard` |
| **Auth & Users** | Wallet | `InitWallet` |
| **Blackjack Game** | Wallet | `Hold`, `Settle`, `Release` |
| **Blackjack Game** | History & Audit | `RecordGameEvent` |
| **Poker Game** | Wallet | `Hold`, `Settle` |
| **Poker Game** | History & Audit | `RecordGameEvent` |
| **History & Audit** | Leaderboard & Stats | `StreamGameEvents` |

---

### 5. Resumen de Bases de Datos por Servicio

| Servicio | Base de Datos | Motor | Recurso principal |
|---|---|---|---|
| Auth & Users | `users_db` | PostgreSQL | Tablas: `users`, `sessions` |
| Wallet | `wallet_db` | PostgreSQL | Tablas: `wallets`, `holds`, `transactions` |
| Blackjack Game | `bj_state_db` | MongoDB | Colección: `hands` |
| Poker Game | `poker_state_db` | MongoDB | Colecciones: `tables`, `rounds` |
| History & Audit | `audit_history_db` | MongoDB | Colección: `game_events` |
| Leaderboard & Stats | `stats_db` | MongoDB | Colecciones: `player_stats`, `player_game_stats`, `rankings` |
