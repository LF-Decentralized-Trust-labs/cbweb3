FROM node:22-alpine AS builder

WORKDIR /workspace/frontend

COPY package*.json ./
COPY apps/governance/package.json apps/governance/package.json
COPY packages/config/package.json packages/config/package.json
COPY packages/ui/package.json packages/ui/package.json

RUN npm ci

COPY . .

ARG VITE_API_URL
ENV VITE_API_URL=${VITE_API_URL}

ARG VITE_PORTAL_OWNER
ENV VITE_PORTAL_OWNER=${VITE_PORTAL_OWNER}

ARG VITE_FIAT_SYMBOL
ENV VITE_FIAT_SYMBOL=${VITE_FIAT_SYMBOL}

RUN npm run build --workspace=governance

FROM nginx:1.27-alpine

COPY --from=builder /workspace/frontend/apps/governance/dist /usr/share/nginx/html
COPY nginx/spa.conf /etc/nginx/conf.d/default.conf

EXPOSE 80

CMD ["nginx", "-g", "daemon off;"]