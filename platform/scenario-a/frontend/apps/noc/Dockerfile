FROM node:22-alpine AS builder

WORKDIR /workspace/frontend

COPY package*.json ./
COPY apps/noc/package.json apps/noc/package.json
COPY packages/config/package.json packages/config/package.json
COPY packages/ui/package.json packages/ui/package.json

RUN npm ci

COPY . .

ARG VITE_NOC_BACKEND_URL
ENV VITE_NOC_BACKEND_URL=${VITE_NOC_BACKEND_URL}

ARG VITE_KEYCLOAK_URL
ENV VITE_KEYCLOAK_URL=${VITE_KEYCLOAK_URL}

ARG VITE_KEYCLOAK_REALM
ENV VITE_KEYCLOAK_REALM=${VITE_KEYCLOAK_REALM}

ARG VITE_KEYCLOAK_CLIENT_ID
ENV VITE_KEYCLOAK_CLIENT_ID=${VITE_KEYCLOAK_CLIENT_ID}

RUN npm run build --workspace=noc

FROM nginx:1.27-alpine

COPY --from=builder /workspace/frontend/apps/noc/dist /usr/share/nginx/html
COPY nginx/spa.conf /etc/nginx/conf.d/default.conf

EXPOSE 80

CMD ["nginx", "-g", "daemon off;"]
