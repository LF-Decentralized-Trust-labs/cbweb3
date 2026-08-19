FROM golang:1.26-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /noc-backend ./cmd/server

# `:nonroot` rather than the bare tag: the default distroless image runs as root, which
# is the other half of finding R2-M-12 — the card cites these two as the good pattern
# to follow, but they carried no USER either. The variant pins the uid to 65532.
# A digest would pin the content too; the tag is what upstream publishes.
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=builder /noc-backend /noc-backend
EXPOSE 8090
USER 65532:65532

ENTRYPOINT ["/noc-backend"]
