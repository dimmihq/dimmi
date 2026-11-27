# Caddy for every environment's edge, with two modules. The Vercel DNS
# module lets an edge order a wildcard certificate: a certificate for every
# name under a domain can only be proved through the domain's DNS, and
# dimmi's DNS is at Vercel. The geoip2 module, with DB-IP's free country
# database (CC BY 4.0), tells a service behind the edge which country a
# visitor is in. The edge image workflow (.github/workflows/edge-image.yml)
# builds this and pushes it under the tag the base edge names. Change a
# version here and that tag together; a new tag also fetches the month's
# database.
ARG CADDY_VERSION=2.10.2

FROM caddy:${CADDY_VERSION}-builder-alpine AS build
RUN xcaddy build \
    --with github.com/caddy-dns/vercel@90ab6e6ed7 \
    --with github.com/zhangjiayin/caddy-geoip2@v1.4.0

# DB-IP publishes each month's file early in the month; until it does, the
# previous month's is the latest. A failed fetch fails the build.
FROM alpine:3.22 AS geoip
RUN set -eu; \
    year="$(date -u +%Y)"; month="$(date -u +%m)"; month="${month#0}"; \
    if [ "$month" = 1 ]; then previous="$((year - 1))-12"; \
    else previous="$(printf '%d-%02d' "$year" "$((month - 1))")"; fi; \
    for month in "$(date -u +%Y-%m)" "$previous"; do \
      if wget -q --tries=3 -O /tmp/country.mmdb.gz \
        "https://download.db-ip.com/free/dbip-country-lite-$month.mmdb.gz"; then break; fi; \
      rm -f /tmp/country.mmdb.gz; \
    done; \
    mkdir -p /geoip; \
    gunzip -c /tmp/country.mmdb.gz > /geoip/dbip-country-lite.mmdb

FROM caddy:${CADDY_VERSION}-alpine
COPY --from=build /usr/bin/caddy /usr/bin/caddy
COPY --from=geoip /geoip /usr/share/geoip
