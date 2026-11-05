FROM node:26-alpine AS builder

WORKDIR /workspace/frontend

COPY package*.json ./
COPY apps/treasury/package.json apps/treasury/package.json
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

ARG VITE_INSTITUTION_NAME
ENV VITE_INSTITUTION_NAME=${VITE_INSTITUTION_NAME}

ARG VITE_BASE_PATH
ENV VITE_BASE_PATH=${VITE_BASE_PATH}
ARG VITE_LAUNCHER_URL
ENV VITE_LAUNCHER_URL=${VITE_LAUNCHER_URL}

RUN npm run build --workspace=treasury

FROM nginx:1.31-alpine

COPY --from=builder /workspace/frontend/apps/treasury/dist /usr/share/nginx/html
COPY nginx/spa.conf /etc/nginx/conf.d/default.conf

EXPOSE 80

CMD ["nginx", "-g", "daemon off;"]
