# HN Reader Cloud — runtime image for h43 (Rocky Linux 9 + Docker).
#
# The binary is cross-compiled on the workstation and uploaded next to this
# file; building Go inside the 1.9 GB h43 host is unnecessary and risks OOM.
#
#   cd server
#   CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -tags cloud -trimpath \
#     -ldflags "-s -w" -o hn-reader-cloud ./cmd/hn-reader-cloud
#
# Then on h43:
#   cd /root/hn-reader && docker build -t hn-reader-cloud:1.3.2 .
FROM alpine:3.20

RUN apk add --no-cache ca-certificates \
 && addgroup -S app \
 && adduser -S -G app -u 10001 app

COPY hn-reader-cloud /usr/local/bin/hn-reader-cloud

USER app
EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
  CMD wget -qO- http://127.0.0.1:8080/health || exit 1

ENTRYPOINT ["/usr/local/bin/hn-reader-cloud"]
