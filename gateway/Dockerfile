FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY go.mod go.sum* ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -ldflags="-w -s" -o /gateway ./cmd/gateway

FROM alpine:3.20
# alpine plutôt que scratch : le healthcheck (docker-compose.yml) exécute
# `wget` à l'intérieur du container pour sonder /health — scratch n'a
# aucun binaire du tout, le healthcheck échouait systématiquement bien
# que le gateway tourne normalement (confirmé : logs propres, healthcheck
# rouge quand même).
RUN apk add --no-cache ca-certificates
COPY --from=builder /gateway /gateway
EXPOSE 8080 9090
ENTRYPOINT ["/gateway"]
