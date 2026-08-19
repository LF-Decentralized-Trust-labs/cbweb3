FROM golang:1.26-alpine AS builder
RUN apk add --no-cache gcc musl-dev
WORKDIR /workspace
COPY shared/ shared/
COPY services/auth/ services/auth/
WORKDIR /workspace/services/auth
RUN go mod download
RUN go build -o /app/service ./cmd/auth

FROM alpine:latest
RUN apk --no-cache add ca-certificates
COPY --from=builder /app/service /app/service
ENTRYPOINT ["/app/service"]
