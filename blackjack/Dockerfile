# Multi-stage Dockerfile para Blackjack Game Service
FROM golang:alpine AS builder

WORKDIR /app

# Copiar definiciones de dependencias locales referenciadas
COPY wallet/go.mod wallet/go.sum ./wallet/
COPY wallet/pb ./wallet/pb/

COPY blackjack/go.mod blackjack/go.sum ./blackjack/
WORKDIR /app/blackjack
RUN go mod download

# Copiar el resto del código del servicio
COPY blackjack .

# Compilación estática
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /app/blackjack-service .

# Imagen final ligera y segura
FROM alpine:3.20

RUN apk --no-cache add ca-certificates tzdata

WORKDIR /app

COPY --from=builder /app/blackjack-service /app/blackjack-service

EXPOSE 50052

ENTRYPOINT ["/app/blackjack-service"]

