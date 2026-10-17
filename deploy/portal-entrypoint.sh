#!/bin/sh
# Renders the portal page from its template with this deployment's download
# settings, then hands over to nginx.
#
# The page is a plain file carrying ${PORTAL_...} placeholders. nginx's own
# envsubst-on-templates is not used: it can only leave an unset value empty,
# never drop the line, which would leave an empty checksum row on screen. So
# the substitution happens here, and an element marked data-optional whose
# value is empty is removed whole.
set -eu

template=/usr/share/nginx/html/index.html.template
out=/usr/share/nginx/html/index.html

# The public releases page is the fallback: a deployment that configures
# nothing still ships a page whose download button works, just slowly from
# outside China. Set these to your own server or CDN for the real thing.
default_releases="https://github.com/mbfczzz/FlowLight-Accelerator/releases"
: "${PORTAL_VERSION:=}"
: "${PORTAL_FILENAME:=}"
: "${PORTAL_SHA256:=}"
: "${PORTAL_RELEASES_URL:=$default_releases}"
: "${PORTAL_DOWNLOAD_URL:=$default_releases/latest}"
export PORTAL_VERSION PORTAL_FILENAME PORTAL_SHA256 PORTAL_RELEASES_URL PORTAL_DOWNLOAD_URL

# A literal replacement: escape what sed would otherwise read as syntax.
esc() {
 printf '%s' "$1" | sed -e 's/[\\&|]/\\&/g'
}

render() {
 sed \
  -e "s|\${PORTAL_VERSION}|$(esc "$PORTAL_VERSION")|g" \
  -e "s|\${PORTAL_FILENAME}|$(esc "$PORTAL_FILENAME")|g" \
  -e "s|\${PORTAL_SHA256}|$(esc "$PORTAL_SHA256")|g" \
  -e "s|\${PORTAL_DOWNLOAD_URL}|$(esc "$PORTAL_DOWNLOAD_URL")|g" \
  -e "s|\${PORTAL_RELEASES_URL}|$(esc "$PORTAL_RELEASES_URL")|g" \
  "$template"
}

# Drop a marked element that ended up with nothing in it. Each marker wraps
# one span or div on its own line of the template, so dropping the line is
# enough, and the page never shows an empty version or checksum.
drop_empty() {
 awk -v version="$PORTAL_VERSION" -v sha="$PORTAL_SHA256" '
  /data-optional="version"/ && version == "" { next }
  /data-optional="sha"/ && sha == "" { next }
  { print }
 '
}

render | drop_empty > "$out"

# A missing placeholder in the template would silently ship as an empty link.
if grep -q '\${PORTAL_' "$out"; then
 echo "portal: unsubstituted placeholder left in the page" >&2
 grep -o '\${PORTAL_[A-Z_]*}' "$out" | sort -u >&2
 exit 1
fi
if [ ! -s "$out" ]; then
 echo "portal: the rendered page is empty; refusing to serve it" >&2
 exit 1
fi

echo "portal: page rendered for $PORTAL_VERSION (download from $PORTAL_DOWNLOAD_URL)"
# The nginx image's own entrypoint renders the server configuration from
# /etc/nginx/templates (the listen address, the CSP, the /dist/ hint), then
# runs the command: nginx in the foreground by default, or what was asked
# for, such as nginx -t. Starting nginx here directly would skip that step
# and serve the stock configuration on port 80 instead.
exec /docker-entrypoint.sh "$@"
