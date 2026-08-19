FROM golang:1.26-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /noc-backend ./cmd/server

FROM gcr.io/distroless/static-debian12
COPY --from=builder /noc-backend /noc-backend
EXPOSE 8091
ENTRYPOINT ["/noc-backend"]
