FROM node:24-alpine AS build
WORKDIR /src
COPY apps/admin-web/package*.json ./
RUN npm ci
COPY apps/admin-web/ .
RUN npm run build
FROM nginx:1.28-alpine
RUN apk add --no-cache ca-certificates
COPY --from=build /src/dist /usr/share/nginx/html
COPY deploy/admin.nginx.template /etc/nginx/templates/default.conf.template
ENV ADMIN_LISTEN=80 API_UPSTREAM=http://control-api:8090 CONTROL_TLS_SERVER_NAME=control-api REAL_IP_FROM=127.0.0.1
EXPOSE 80
