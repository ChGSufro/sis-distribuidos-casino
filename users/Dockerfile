# syntax=docker/dockerfile:1
FROM golang:1.27-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -o /out/users ./cmd/users

FROM alpine:3.21
RUN adduser -D -u 10001 appuser
COPY --from=build /out/users /users
USER appuser
EXPOSE 50056
ENTRYPOINT ["/users"]
