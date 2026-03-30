# ---- Build stage ----
FROM golang:1.25-alpine AS build

ARG VERSION=dev

WORKDIR /src

# Cache module downloads before copying full source
COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Static binary — no cgo, no external deps at runtime
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w -X main.version=$VERSION" -o /bin/tg-replier .

# ---- Runtime stage ----
FROM alpine:3.21

# CA certs for outbound HTTPS to the Telegram API
RUN apk add --no-cache ca-certificates

# Non-root user
RUN adduser -D -u 1000 appuser

# Data directory with correct ownership
RUN mkdir -p /app/data && chown appuser:appuser /app/data

WORKDIR /app

COPY --from=build /bin/tg-replier /app/tg-replier

# Default data path inside the container (mount a volume here)
ENV DATA_DIR=/app/data

USER appuser

ENTRYPOINT ["/app/tg-replier"]
