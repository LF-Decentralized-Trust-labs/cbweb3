# Build stage
FROM node:20-alpine AS builder
WORKDIR /app

# Copy proto file needed at runtime by @grpc/proto-loader
COPY apis/proto/payment_orchestrator/v1/payment_orchestrator.proto \
     apis/proto/payment_orchestrator/v1/payment_orchestrator.proto

# Build tools required by native addons (bufferutil, utf-8-validate)
RUN apk add --no-cache python3 make g++

# Install dependencies
COPY interop/hub-and-spoke/cacti/package.json \
     interop/hub-and-spoke/cacti/tsconfig.json \
     ./
RUN npm config set fetch-timeout 300000 && \
    npm config set fetch-retries 5 && \
    npm config set fetch-retry-mintimeout 20000 && \
    npm config set fetch-retry-maxtimeout 120000 && \
    npm install --no-audit --prefer-offline || \
    (sleep 10 && npm install --no-audit --prefer-offline) || \
    (sleep 20 && npm install --no-audit)

# Compile TypeScript
COPY interop/hub-and-spoke/cacti/src/ ./src/
RUN npx tsc

# ── Runtime stage ──────────────────────────────────────────────────────────
FROM node:20-alpine
WORKDIR /app

RUN apk add --no-cache ca-certificates

# Copy proto (needed at runtime for gRPC proto-loader)
COPY --from=builder /app/apis/ ./apis/

# Copy compiled JS + production deps
COPY --from=builder /app/dist ./dist
COPY --from=builder /app/node_modules ./node_modules
COPY --from=builder /app/package.json ./package.json

ENV NODE_ENV=production

ENTRYPOINT ["node", "dist/index.js"]
