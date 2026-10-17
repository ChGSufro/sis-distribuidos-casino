# Multi-stage Dockerfile para API Gateway
FROM golang:alpine AS builder

WORKDIR /app

# Copiar dependencias y descargar
COPY go.mod go.sum ./
RUN go mod download

# Copiar código fuente
COPY . .

# Compilación estática
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /app/gateway-service ./cmd/gateway

# Imagen final ligera y segura
FROM alpine:3.20

RUN apk --no-cache add ca-certificates tzdata

WORKDIR /app

COPY --from=builder /app/gateway-service /app/gateway-service

EXPOSE 8080

ENTRYPOINT ["/app/gateway-service"]
