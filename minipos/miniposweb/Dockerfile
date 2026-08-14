FROM node:22.18.0-alpine AS build
WORKDIR /app
COPY package*.json ./
RUN npm install --ignore-scripts --no-audit --no-fund
COPY . .
RUN npm run build
RUN npm run deployment
FROM caddy:2.10.0-alpine
COPY --from=build /app/dist /srv
COPY Caddyfile /etc/caddy/Caddyfile
