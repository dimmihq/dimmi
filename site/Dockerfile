# The marketing site and its docs (/docs) as one set of static files
# behind nginx. Build from the repository root, since both share the
# app's BoardUI components and tokens and the docs read the API contract:
#   docker build -f site/Dockerfile -t dimmi-site .
FROM --platform=$BUILDPLATFORM node:24.11-bookworm-slim AS build
WORKDIR /app
RUN corepack enable
COPY package.json pnpm-lock.yaml pnpm-workspace.yaml ./
RUN pnpm install --frozen-lockfile --ignore-scripts
COPY resources/js resources/js
COPY tsconfig.json ./
COPY tests/contracts/api_v1.openapi.json tests/contracts/
COPY site site
COPY docs docs
RUN pnpm site:build

FROM nginx:1.29-alpine
COPY site/nginx.conf /etc/nginx/conf.d/default.conf
COPY --from=build /app/site/dist /usr/share/nginx/html
EXPOSE 8080
