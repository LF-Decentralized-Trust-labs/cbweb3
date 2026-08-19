FROM golang:1.26-alpine AS builder
RUN apk add --no-cache gcc musl-dev
WORKDIR /workspace
COPY shared/ shared/
COPY services/payment-orchestrator/ services/payment-orchestrator/
WORKDIR /workspace/services/payment-orchestrator
RUN go mod download
RUN go build -o /app/service ./cmd/payment-orchestrator

# Pinned, not `latest`: `latest` makes the image a moving target, so the same tag can
# resolve to different content on two machines and a rebuild can pull in changes nobody
# reviewed. 3.20 is the version already used across this repository (finding R2-M-12).
FROM alpine:3.20
RUN apk --no-cache add ca-certificates
COPY --from=builder /app/service /app/service
# Non-root by default (finding R2-M-12). A fixed uid so a deployment can reason about
# file ownership on mounted paths; the toolkit overrides it with the host user for the
# services that write into a host-owned bind mount (compose `user:`, the pattern
# ADR-001 already established for the Besu containers).
RUN addgroup -g 10001 -S app && adduser -u 10001 -S app -G app
USER 10001:10001

ENTRYPOINT ["/app/service"]
