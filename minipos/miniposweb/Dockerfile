FROM node:22.18.0-alpine AS build
WORKDIR /app/miniposweb
COPY miniposweb/package*.json ./
RUN npm install --ignore-scripts --no-audit --no-fund
COPY miniposweb/ ./
COPY imgs/background.png ../imgs/background.png
RUN npm run build
RUN npm run deployment
FROM caddy:2.10.0-alpine
COPY --from=build /app/miniposweb/dist /srv
COPY miniposweb/Caddyfile /etc/caddy/Caddyfile
