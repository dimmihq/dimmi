# The marketing site and its docs (/docs) as one set of static files
# behind nginx. Build from the repository root, since both share the
# app's BoardUI components and tokens and the docs read the API contract:
#   docker build -f site/Dockerfile -t dimmi-site .
#
# CI builds the files on the runner instead, where pnpm's store is warm,
# and only packs them here:
#   docker buildx build -f site/Dockerfile --build-arg SITE=prebuilt \
#     --build-context prebuilt=site/dist .

ARG SITE=source

FROM --platform=$BUILDPLATFORM node:24.11-bookworm-slim AS build
WORKDIR /app
RUN corepack enable
COPY package.json pnpm-lock.yaml pnpm-workspace.yaml ./
RUN pnpm install --frozen-lockfile --ignore-scripts
COPY resources/js resources/js
COPY tsconfig.json ./
COPY tests/contracts/api_v1.openapi.json tests/contracts/
COPY scripts/generate-sitemap.ts scripts/generate-llms.ts scripts/
COPY sdk/angular/package.json sdk/angular/
COPY sdk/api/package.json sdk/api/
COPY sdk/browser/package.json sdk/browser/
COPY sdk/go/go.mod sdk/go/
COPY sdk/laravel/composer.json sdk/laravel/
COPY sdk/nextjs/package.json sdk/nextjs/
COPY sdk/php/composer.json sdk/php/
COPY sdk/python/pyproject.toml sdk/python/
COPY sdk/react/package.json sdk/react/
COPY sdk/server/package.json sdk/server/
COPY sdk/svelte/package.json sdk/svelte/
COPY sdk/vue/package.json sdk/vue/
COPY site site
COPY docs docs
RUN pnpm site:build

FROM scratch AS source-site
COPY --from=build /app/site/dist /dist

# Empty unless the build replaces it with --build-context prebuilt=<dist>.
FROM scratch AS prebuilt
FROM scratch AS prebuilt-site
COPY --from=prebuilt . /dist

FROM ${SITE}-site AS release-site

# nginx as an unprivileged user, on port 8080.
FROM nginxinc/nginx-unprivileged:1.29-alpine
ARG REVISION=unknown
LABEL org.opencontainers.image.source=https://github.com/dimmihq/dimmi
LABEL org.opencontainers.image.revision=$REVISION
COPY site/nginx.conf /etc/nginx/conf.d/default.conf
COPY --from=release-site /dist /usr/share/nginx/html
EXPOSE 8080
