FROM oven/bun:1 AS web-builder

WORKDIR /src/web

COPY web/package.json web/bun.lock ./
RUN bun install --frozen-lockfile

COPY web/ ./
RUN bun run build

FROM golang:latest AS go-builder

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
COPY --from=web-builder /src/web/dist ./internal/platform/webui/dist
RUN CGO_ENABLED=0 GOOS=linux go build -tags webui -trimpath -ldflags="-s -w" -o /out/payment-gateway ./cmd/server

FROM alpine:latest

RUN addgroup -S app && adduser -S app -G app

WORKDIR /app
COPY --from=go-builder /out/payment-gateway /app/payment-gateway

USER app
EXPOSE 8080

ENTRYPOINT ["/app/payment-gateway"]
