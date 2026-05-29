FROM golang:1.25-alpine AS builder
WORKDIR /workspace
COPY shared/ shared/
COPY services/api-gateway/ services/api-gateway/
WORKDIR /workspace/services/api-gateway
RUN go mod download
RUN CGO_ENABLED=0 go build -o /app/service ./cmd/api-gateway

FROM alpine:latest
RUN apk --no-cache add ca-certificates
COPY --from=builder /app/service /app/service
ENTRYPOINT ["/app/service"]
