# syntax=docker/dockerfile:1
# Multi-stage Dockerfile para Poker Game Service.
# El contexto de build es la raiz del repo: go.mod referencia el modulo
# local de wallet con replace => ../wallet.
FROM golang:1.27-alpine AS build

WORKDIR /app

# Dependencia local (wallet) referenciada por replace
COPY wallet/go.mod wallet/go.sum ./wallet/
COPY wallet/pb ./wallet/pb/

COPY poker/go.mod poker/go.sum ./poker/
WORKDIR /app/poker
RUN go mod download

# Codigo del servicio
COPY poker .

RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/poker ./cmd/poker

FROM alpine:3.21
RUN apk --no-cache add ca-certificates tzdata \
	&& adduser -D -u 10001 appuser
COPY --from=build /out/poker /poker
USER appuser
EXPOSE 50055
ENTRYPOINT ["/poker"]
