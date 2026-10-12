# Multi-stage Dockerfile para Wallet Service
FROM golang:alpine AS builder

WORKDIR /app

# Descarga de dependencias
COPY go.mod go.sum ./
RUN go mod download

# Copiar el código fuente
COPY . .

# Compilación estática
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /app/wallet-service .

# Imagen final ligera y segura
FROM alpine:3.20

RUN apk --no-cache add ca-certificates tzdata

WORKDIR /app

COPY --from=builder /app/wallet-service /app/wallet-service

EXPOSE 50051

ENTRYPOINT ["/app/wallet-service"]
