FROM golang:1.26-alpine AS builder
RUN apk add --no-cache gcc musl-dev
WORKDIR /workspace
COPY shared/ shared/
COPY services/compliance/ services/compliance/
WORKDIR /workspace/services/compliance
RUN go mod download
RUN go build -o /app/service ./cmd/compliance

FROM alpine:latest
RUN apk --no-cache add ca-certificates
COPY --from=builder /app/service /app/service
ENTRYPOINT ["/app/service"]
