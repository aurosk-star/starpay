FROM oven/bun:1.3.13 AS web-builder

WORKDIR /src/web

COPY web/package.json web/bun.lock ./
RUN bun install --frozen-lockfile

COPY web/ ./
RUN bun run build

FROM golang:1.26.6-alpine3.23 AS go-builder

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
COPY --from=web-builder /src/web/dist ./internal/platform/webui/dist
RUN CGO_ENABLED=0 GOOS=linux go build -tags webui -trimpath -ldflags="-s -w" -o /out/payment-gateway ./cmd/server

FROM alpine:3.23

RUN addgroup -S app && adduser -S app -G app

WORKDIR /app
COPY --from=go-builder /out/payment-gateway /app/payment-gateway
COPY LICENSE NOTICE THIRD_PARTY_NOTICES.md /licenses/

USER app
EXPOSE 8080

ENTRYPOINT ["/app/payment-gateway"]
