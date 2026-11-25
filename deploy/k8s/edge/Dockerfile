# Caddy with the Vercel DNS module, for an edge that orders a wildcard
# certificate: a certificate for every name under a domain can only be
# proved through the domain's DNS, and dimmi's DNS is at Vercel. The edge
# image workflow (.github/workflows/edge-image.yml) builds this and pushes
# it under the tag the production edge names. Change a version here and
# that tag together.
ARG CADDY_VERSION=2.10.2

FROM caddy:${CADDY_VERSION}-builder-alpine AS build
RUN xcaddy build \
    --with github.com/caddy-dns/vercel@90ab6e6ed7

FROM caddy:${CADDY_VERSION}-alpine
COPY --from=build /usr/bin/caddy /usr/bin/caddy
