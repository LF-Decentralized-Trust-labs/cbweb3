FROM node:22-alpine AS builder

WORKDIR /workspace/frontend

COPY package*.json ./
COPY apps/supervisor/package.json apps/supervisor/package.json
COPY packages/config/package.json packages/config/package.json
COPY packages/ui/package.json packages/ui/package.json

RUN npm ci

COPY . .

ARG VITE_API_BASE_URL
ENV VITE_API_BASE_URL=${VITE_API_BASE_URL}

ARG VITE_INSTITUTION_NAME
ENV VITE_INSTITUTION_NAME=${VITE_INSTITUTION_NAME}

ARG VITE_LAUNCHER_URL
ENV VITE_LAUNCHER_URL=${VITE_LAUNCHER_URL}

RUN npm run build --workspace=supervisor

FROM nginx:1.27-alpine

COPY --from=builder /workspace/frontend/apps/supervisor/dist /usr/share/nginx/html
COPY nginx/spa.conf /etc/nginx/conf.d/default.conf

EXPOSE 80

CMD ["nginx", "-g", "daemon off;"]
