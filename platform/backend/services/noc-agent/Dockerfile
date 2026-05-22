FROM golang:1.25.5-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /noc-agent ./cmd/agent

FROM gcr.io/distroless/static-debian12
COPY --from=builder /noc-agent /noc-agent
ENTRYPOINT ["/noc-agent"]
