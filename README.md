# Casino Distribuido — Sistemas Distribuidos

Plataforma de casino en línea construida como sistema distribuido de microservicios. Ofrece juegos de **Poker** y **Blackjack**, cartera con ledger de saldos, autenticación JWT, historial de partidas y leaderboard.

## Arquitectura

- **7 microservicios en Go**: API Gateway, Users/Auth, Wallet, Poker, Blackjack, History y Stats.
- **Database-per-Service**: 2× PostgreSQL (users, wallet) y 4× MongoDB (poker, blackjack, history, stats).
- **API Gateway** (`:8080`) como único punto de entrada HTTP/REST; traduce a gRPC internamente.
- Todos los servicios corren en la red bridge `casino-net` y se comunican por nombre de servicio.

| Servicio   | Puerto | BD          | Propósito                          |
| ---------- | ------ | ----------- | ---------------------------------- |
| gateway    | 8080   | —           | Punto de entrada REST → gRPC       |
| users      | 50056  | PostgreSQL  | Registro, login y sesiones JWT     |
| wallet     | 50051  | PostgreSQL  | Saldos, depósitos y holds/ledger   |
| poker      | 50055  | MongoDB     | Mesas y estado de partidas         |
| blackjack  | 50052  | MongoDB     | Manos y estado del juego           |
| history    | 50053  | MongoDB     | Auditoría e historial de eventos   |
| stats      | 50054  | MongoDB     | Estadísticas y leaderboard         |

Documentación de diseño detallada: [`docs/informe-plan.md`](docs/informe-plan.md).

## Levantar el proyecto

Requisitos: Docker con Compose v2. Puertos requeridos: `8080`, `50051-50056`, `5434`, `27017`.

```bash
cp .env.example .env        # opcional: todos los valores tienen defaults
docker compose up --build -d
curl localhost:8080/health  # → {"status":"healthy","service":"api-gateway"}
```

La primera ejecución construye las imágenes de los 7 servicios y puede tardar unos minutos.

## Usar la UI

La interfaz web está en [`frontend/`](frontend/) (HTML/CSS/JS estático, sin build). Se comunica con el API Gateway en `http://localhost:8080`.

```bash
# 1. Levantar el backend
docker compose up --build -d

# 2. Servir la UI (requiere Python 3)
cd frontend && python3 -m http.server 8000
```

Abrir **http://localhost:8000** → `index.html`.

### Flujo básico

1. **Registro / Login** (`index.html`): crear cuenta (usuario, email, contraseña) e iniciar sesión. El token JWT se guarda en `localStorage` y se renueva automáticamente.
2. **Menú superior**: Blackjack | Póker | Historial | Estadísticas | Salir.
3. **Blackjack** (`game.html`): depositar saldo en la barra superior → crear mano con tu apuesta → `hit`, `stand`, `double`, `split` o `surrender`.
4. **Póker** (`poker.html`): crear mesa (ciegas, máx. jugadores, buy-in opcional) o unirse a una existente → en la mesa: `fold`, `check`, `call`, `raise` o `all-in`. Las fichas son reales (wallet): el saldo se reserva al entrar y se liquida al salir.
5. **Historial** (`history.html`): partidas con filtro por juego (blackjack/póker) y paginación.
6. **Estadísticas** (`stats.html`): total apostado, ganado, beneficio neto, partidas jugadas y win rate.
7. **Salir**: cierra la sesión (logout) y limpia `localStorage`.

## Casos de uso con curl

Verificación end-to-end de los 4 casos de uso contra el gateway. Requiere `jq` y el stack levantado (`docker compose up -d`).

```bash
BASE_URL=http://localhost:8080
```

### Caso de uso 1: Registrar cuenta

Crea la cuenta del jugador en el servicio de usuarios e inicializa automáticamente su billetera en el servicio de wallet. Posteriormente se obtiene el token JWT para autorizar los casos de uso siguientes.

```bash
# 1.1 Registrar nuevo usuario
REGISTER_RESP=$(curl -s -X POST "$BASE_URL/api/auth/register" \
  -H "Content-Type: application/json" \
  -d '{
    "username": "jugador1",
    "email": "jugador1@casino.local",
    "password": "Password123!"
  }')
echo "$REGISTER_RESP" | jq .

# 1.2 Iniciar sesión y guardar token JWT
LOGIN_RESP=$(curl -s -X POST "$BASE_URL/api/auth/login" \
  -H "Content-Type: application/json" \
  -d '{
    "email": "jugador1@casino.local",
    "password": "Password123!"
  }')
echo "$LOGIN_RESP" | jq .

TOKEN=$(echo "$LOGIN_RESP" | jq -r .data.access_token)
```

### Caso de uso 2: Procesar depósito de saldo

Permite al jugador consultar su saldo inicial y abonar fondos a su billetera.

```bash
# 2.1 Consultar saldo inicial
curl -s -X GET "$BASE_URL/api/wallet/balance" \
  -H "Authorization: Bearer $TOKEN" | jq .

# 2.2 Procesar depósito de $100.00 (10,000 centavos)
curl -s -X POST "$BASE_URL/api/wallet/deposit" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "amount_cents": 10000
  }' | jq .

# 2.3 Confirmar nuevo saldo disponible
curl -s -X GET "$BASE_URL/api/wallet/balance" \
  -H "Authorization: Bearer $TOKEN" | jq .
```

### Caso de uso 3: Procesar partida de Blackjack

Abarca el ciclo completo de juego: colocación de la apuesta (reserva/hold en wallet), consulta de cartas y toma de decisiones (hit o stand), finalizando con la liquidación automática de ganancias o pérdidas.

```bash
# 3.1 Crear mano e iniciar partida con apuesta de $10.00 (1,000 centavos)
BJ_RESP=$(curl -s -X POST "$BASE_URL/api/games/blackjack/hands" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "bet_amount_cents": 1000
  }')
echo "$BJ_RESP" | jq .

HAND_ID=$(echo "$BJ_RESP" | jq -r .data.hand_id)

# 3.2 Consultar estado de la mano
curl -s -X GET "$BASE_URL/api/games/blackjack/hands/$HAND_ID" \
  -H "Authorization: Bearer $TOKEN" | jq .

# 3.3 Acción: Pedir carta (hit) - Opcional según cartas iniciales
curl -s -X POST "$BASE_URL/api/games/blackjack/hands/$HAND_ID/action" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "sub_hand_index": 0,
    "action": "hit"
  }' | jq .

# 3.4 Acción: Plantarse (stand) para concluir la mano y liquidar saldo
curl -s -X POST "$BASE_URL/api/games/blackjack/hands/$HAND_ID/action" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "sub_hand_index": 0,
    "action": "stand"
  }' | jq .
```

### Caso de uso 4: Obtener historial y estadísticas

Permite al jugador auditar sus partidas jugadas y revisar sus métricas de rendimiento acumuladas procesadas de forma asíncrona.

```bash
# 4.1 Obtener historial de partidas / eventos de auditoría
curl -s -X GET "$BASE_URL/api/history?limit=10" \
  -H "Authorization: Bearer $TOKEN" | jq .

# 4.2 Obtener estadísticas globales del jugador (ganadas, perdidas, balance neto)
curl -s -X GET "$BASE_URL/api/stats/player" \
  -H "Authorization: Bearer $TOKEN" | jq .

# 4.3 Obtener estadísticas específicas de Blackjack
curl -s -X GET "$BASE_URL/api/stats/player/blackjack" \
  -H "Authorization: Bearer $TOKEN" | jq .

# 4.4 Consultar ranking / leaderboard global
curl -s -X GET "$BASE_URL/api/stats/leaderboard?period=all_time&limit=5" | jq .
```

## Comandos Docker útiles para evaluación

### Estado y salud

```bash
docker compose ps                          # estado de todos los servicios + healthchecks
docker compose ps --format json             # estado en JSON (para scripts)
docker inspect --format '{{json .State.Health}}' wallet | jq   # healthcheck detallado
```

### Logs (flujo distribuido)

```bash
docker compose logs -f gateway             # seguir logs de un servicio
docker compose logs --tail 100 poker       # últimas 100 líneas
docker compose logs --since 5m             # logs de los últimos 5 minutos
docker compose logs | grep -i error        # buscar errores en todos los servicios
```

### Consumo de recursos

```bash
docker compose stats                       # CPU/memoria en vivo de todos los contenedores
docker compose top                         # procesos internos de cada contenedor
```

### Red y comunicación entre servicios

```bash
docker network inspect casino-net          # contenedores conectados y aliases
docker compose exec gateway ping -c 3 users # resolución de nombres por servicio
```

### Inspección de bases de datos (Database-per-Service)

```bash
docker compose exec postgres-users psql -U casino -d users_db -c '\dt'
docker compose exec wallet_db psql -U casino_user -d wallet_db -c 'SELECT * FROM wallets LIMIT 5;'
docker compose exec mongo-poker mongosh --eval 'db.tables.find().limit(5)'
docker compose exec mongo-history mongosh --eval 'db.events.find().limit(5)'
```

### Resiliencia y tolerancia a fallos

```bash
docker compose restart users                # reiniciar un servicio
docker compose stop blackjack && docker compose start blackjack
docker compose kill history                 # simular fallo abrupto
# Con restart: unless-stopped, el servicio se levanta solo tras un fallo
docker compose ps                           # verificar que se recuperó
```

### Volúmenes (persistencia)

```bash
docker volume ls                           # volúmenes del proyecto
docker volume inspect sis-distribuidos-casino_users-pg-data
docker compose down -v                      # ⚠️ baja TODO y borra datos persistentes
```

### Validación y limpieza

```bash
docker compose config                       # validar docker-compose.yml + .env
docker compose down                         # detener sin borrar volúmenes
docker compose up --build -d                # reconstruir tras cambios de código
```
