FROM node:22-alpine AS builder

WORKDIR /app

COPY package*.json ./
RUN npm install

COPY . .

# The image is GENERIC: no per-entity URLs are baked in. Each entity's portal list is
# provided at runtime via config.json (see gen-config.sh), served next to index.html.
RUN npm run build

FROM nginx:1.27-alpine

COPY --from=builder /app/dist /usr/share/nginx/html
COPY nginx/spa.conf /etc/nginx/conf.d/default.conf

# Per-scenario config fragments: the build ships placeholders under public/configs/.
# Each entity's deploy mounts a dir of real fragments over /usr/share/nginx/html/configs,
# one config.<scenario>.json per deployed scenario (written by each scenario's toolkit):
#   docker run -p 5190:80 \
#     -v "$LAUNCHER_STATE_DIR/configs:/usr/share/nginx/html/configs:ro" cbweb3/launcher
# The toolkits (launcher: enable) do this automatically.

EXPOSE 80

CMD ["nginx", "-g", "daemon off;"]
