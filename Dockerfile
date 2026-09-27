# syntax=docker/dockerfile:1

# ---- build stage ----
FROM golang:1.23-alpine AS build
WORKDIR /src
COPY go.mod ./
RUN go mod download 2>/dev/null || true
COPY . .
ARG VERSION=1.0.0
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath \
    -ldflags="-s -w -X github.com/gitstq/nostromo-relay/internal/server.Version=${VERSION}" \
    -o /out/nostromo ./cmd/nostromo

# ---- minimal runtime ----
FROM scratch
LABEL org.opencontainers.image.title="Nostromo" \
      org.opencontainers.image.description="Zero-dependency Nostr event-log relay" \
      org.opencontainers.image.licenses="MIT" \
      org.opencontainers.image.source="https://github.com/gitstq/nostromo-relay"
COPY --from=build /out/nostromo /nostromo
EXPOSE 7777
VOLUME ["/data"]
# Memory-only by default; mount a volume and pass --db to persist.
ENTRYPOINT ["/nostromo", "serve", "--addr", ":7777", "--db", "/data/events.jsonl"]
