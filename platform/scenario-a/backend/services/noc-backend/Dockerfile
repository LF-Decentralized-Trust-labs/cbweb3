FROM golang:1.26-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /noc-backend ./cmd/server

# `:nonroot` rather than the bare tag: the default distroless image runs as root, which
# is the other half of finding R2-M-12 — the card cites these two as the good pattern
# to follow, but they carried no USER either. The variant pins the uid to 65532.
#
# Pinned by digest as well, because `nonroot` is republished on every distroless build:
# on its own it is exactly the moving target this change removes from `alpine:latest`,
# and the hardening guard would have certified it as pinned. Bump the digest knowingly.
FROM gcr.io/distroless/static-debian12:nonroot@sha256:1b7b9f0f0e0a1d2155f531db587cc48ec26aaf97ab64364225f5bf18a054e66a
COPY --from=builder /noc-backend /noc-backend
EXPOSE 8090
USER 65532:65532

ENTRYPOINT ["/noc-backend"]
