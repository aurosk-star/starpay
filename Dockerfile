FROM golang:latest AS builder

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/payment-gateway ./cmd/server

FROM alpine:latest

RUN addgroup -S app && adduser -S app -G app

WORKDIR /app
COPY --from=builder /out/payment-gateway /app/payment-gateway

USER app
EXPOSE 8080

ENTRYPOINT ["/app/payment-gateway"]
