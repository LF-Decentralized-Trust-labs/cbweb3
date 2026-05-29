FROM node:22-alpine AS builder

WORKDIR /workspace/frontend

COPY package*.json ./
COPY apps/dispatcher/package.json apps/dispatcher/package.json
COPY packages/config/package.json packages/config/package.json
COPY packages/ui/package.json packages/ui/package.json

RUN npm ci

COPY . .

ARG VITE_BANK_A_PORTAL_URL
ENV VITE_BANK_A_PORTAL_URL=${VITE_BANK_A_PORTAL_URL}

ARG VITE_BANK_B_PORTAL_URL
ENV VITE_BANK_B_PORTAL_URL=${VITE_BANK_B_PORTAL_URL}

ARG VITE_BANK_C_PORTAL_URL
ENV VITE_BANK_C_PORTAL_URL=${VITE_BANK_C_PORTAL_URL}

ARG VITE_BANK_D_PORTAL_URL
ENV VITE_BANK_D_PORTAL_URL=${VITE_BANK_D_PORTAL_URL}

ARG VITE_CENTRAL_BANK_A_PORTAL_URL
ENV VITE_CENTRAL_BANK_A_PORTAL_URL=${VITE_CENTRAL_BANK_A_PORTAL_URL}

ARG VITE_CENTRAL_BANK_B_PORTAL_URL
ENV VITE_CENTRAL_BANK_B_PORTAL_URL=${VITE_CENTRAL_BANK_B_PORTAL_URL}

RUN npm run build --workspace=dispatcher

FROM nginx:1.27-alpine

COPY --from=builder /workspace/frontend/apps/dispatcher/dist /usr/share/nginx/html
COPY nginx/spa.conf /etc/nginx/conf.d/default.conf

EXPOSE 80

CMD ["nginx", "-g", "daemon off;"]
