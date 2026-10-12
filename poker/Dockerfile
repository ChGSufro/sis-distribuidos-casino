# syntax=docker/dockerfile:1
FROM golang:1.27-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -o /out/poker ./cmd/poker

FROM alpine:3.21
RUN adduser -D -u 10001 appuser
COPY --from=build /out/poker /poker
USER appuser
EXPOSE 50055
ENTRYPOINT ["/poker"]
