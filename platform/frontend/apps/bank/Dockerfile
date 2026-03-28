FROM node:22-alpine AS builder

WORKDIR /workspace/frontend

COPY package.json package-lock.json ./
COPY apps/bank/package.json apps/bank/package.json
COPY packages/config/package.json packages/config/package.json
COPY packages/ui/package.json packages/ui/package.json

RUN npm ci

COPY . .

ARG VITE_API_URL
ENV VITE_API_URL=${VITE_API_URL}

RUN npm run build --workspace=bank

FROM nginx:1.27-alpine

COPY --from=builder /workspace/frontend/apps/bank/dist /usr/share/nginx/html
COPY nginx/spa.conf /etc/nginx/conf.d/default.conf

EXPOSE 80

CMD ["nginx", "-g", "daemon off;"]