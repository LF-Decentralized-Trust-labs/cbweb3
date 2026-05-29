FROM golang:1.25-alpine AS builder
RUN apk add --no-cache gcc musl-dev
WORKDIR /workspace
COPY shared/ shared/
COPY services/payment-orchestrator/ services/payment-orchestrator/
WORKDIR /workspace/services/payment-orchestrator
RUN go mod download
RUN go build -o /app/service ./cmd/payment-orchestrator

FROM alpine:latest
RUN apk --no-cache add ca-certificates
COPY --from=builder /app/service /app/service
ENTRYPOINT ["/app/service"]
