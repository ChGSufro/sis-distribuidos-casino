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
