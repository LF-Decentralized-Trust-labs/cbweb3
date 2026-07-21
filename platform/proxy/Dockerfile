FROM caddy:2-alpine

# Base config only — no per-entity hosts/ports are baked in. Each entity's deploy
# bind-mounts a dir of per-scenario route fragments over /etc/caddy/conf.d (one
# caddy.<scenario>.conf per deployed scenario, written by that scenario's toolkit):
#   docker run -p 80:80 \
#     -v "$PROXY_STATE_DIR/conf.d:/etc/caddy/conf.d:ro" cbweb3/proxy:local
# The toolkits (proxy: enable) do this automatically.
COPY Caddyfile /etc/caddy/Caddyfile

# Empty at build; the mount above supplies the real fragments at runtime. A zero-match
# import glob is not an error, so the proxy also starts cleanly with no fragments yet.
RUN mkdir -p /etc/caddy/conf.d

EXPOSE 80
