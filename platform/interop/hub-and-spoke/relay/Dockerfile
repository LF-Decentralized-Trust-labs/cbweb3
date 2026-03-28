FROM golang:1.25-alpine AS builder
WORKDIR /app
COPY interop/hub-and-spoke/relay/go.mod interop/hub-and-spoke/relay/go.sum ./interop/hub-and-spoke/relay/
COPY backend/shared/proto/ ./backend/shared/proto/
RUN cd interop/hub-and-spoke/relay && go mod download
COPY interop/hub-and-spoke/relay/ ./interop/hub-and-spoke/relay/
RUN cd interop/hub-and-spoke/relay && CGO_ENABLED=0 go build -o /relay .

FROM alpine:3.21
RUN apk add --no-cache ca-certificates
COPY --from=builder /relay /usr/local/bin/relay
ENTRYPOINT ["relay"]
