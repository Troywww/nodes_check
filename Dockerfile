FROM golang:1.22-bookworm AS builder
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY cmd ./cmd
COPY internal ./internal
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/nodes-check ./cmd/server

FROM debian:bookworm-slim
WORKDIR /app
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates tzdata && rm -rf /var/lib/apt/lists/*
COPY --from=builder /out/nodes-check /app/nodes-check
COPY configs/config.yaml /app/defaults/config.yaml
COPY configs/subscription_urls.txt /app/defaults/subscription_urls.txt
COPY bin/xray-linux-64 /app/bin/xray-linux-64
COPY scripts/docker-entrypoint.sh /app/docker-entrypoint.sh
RUN mkdir -p /app/configs /app/runtime/cache /app/runtime/logs /app/runtime/outputs /app/defaults \
    && chmod +x /app/nodes-check /app/bin/xray-linux-64/xray /app/docker-entrypoint.sh
EXPOSE 18808
ENTRYPOINT ["/app/docker-entrypoint.sh"]