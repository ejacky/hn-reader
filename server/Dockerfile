# Build stage
FROM golang:1.26-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -tags cloud -trimpath \
    -ldflags "-s -w" -o server ./cmd/hn-reader-cloud

# Runtime stage
FROM alpine:3.20

RUN apk --no-cache add ca-certificates \
 && addgroup -S app \
 && adduser -S -G app -u 10001 app

WORKDIR /app
COPY --from=builder /app/server /usr/local/bin/hn-reader-cloud

USER app
EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
  CMD wget -qO- http://127.0.0.1:8080/health || exit 1

ENTRYPOINT ["/usr/local/bin/hn-reader-cloud"]
