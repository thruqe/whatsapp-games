# Build stage
FROM golang:1.24-bookworm AS builder

WORKDIR /src

# Cache dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy source and build stripped, optimized binary
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w -buildid=" -v -o /app/bin/server ./cmd/server

# Runtime stage
FROM debian:bookworm-slim

ENV DEBIAN_FRONTEND=noninteractive

RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    curl \
    && rm -rf /var/lib/apt/lists/*

# Install cloudflared for optional public HTTPS quick tunnels
RUN curl -fsSL https://github.com/cloudflare/cloudflared/releases/latest/download/cloudflared-linux-amd64 -o /usr/local/bin/cloudflared \
    && chmod a+rx /usr/local/bin/cloudflared

WORKDIR /app

COPY --from=builder /app/bin/server /app/bin/server

ENV PATH="/app/bin:${PATH}"
WORKDIR /app/bin

EXPOSE 8088

ENTRYPOINT ["server"]
