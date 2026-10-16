# The public portal: one static page (plus its stylesheet), served by nginx.
#
# It carries no scripts and fetches nothing from outside, so it runs under a
# very strict CSP, and it never touches the control service or the database.
#
# The page is a template: deploy/portal-entrypoint.sh fills in the download
# settings from the environment at start-up, so a new client version is a
# compose edit and a restart, not a rebuild of the image.
FROM nginx:1.28-alpine
RUN apk add --no-cache ca-certificates
COPY apps/portal-web/index.html /usr/share/nginx/html/index.html.template
COPY apps/portal-web/portal.css /usr/share/nginx/html/portal.css
COPY deploy/portal-entrypoint.sh /usr/local/bin/portal-entrypoint.sh
COPY deploy/portal.nginx.template /etc/nginx/templates/default.conf.template
RUN chmod 755 /usr/local/bin/portal-entrypoint.sh
ENV PORTAL_LISTEN=80 REAL_IP_FROM=127.0.0.1
EXPOSE 80
ENTRYPOINT ["/usr/local/bin/portal-entrypoint.sh"]
