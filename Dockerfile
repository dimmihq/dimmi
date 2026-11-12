# The cloud binary in the smallest image that runs it: no shell, no package
# manager, a non-root user. It carries CA certificates and time zone data from
# the distroless base, and the RDS certificate bundle so connections verify the
# database. The browser app is built first because the binary embeds it.
#
#   docker buildx build --platform linux/arm64 -t dimmi .
#
# CI builds the binary on the runner instead, where Go's build cache is
# warm, and only packs it here:
#
#   docker buildx build --build-arg BINARY=prebuilt --build-context prebuilt=<dir with dimmi> .

ARG BINARY=source

FROM --platform=$BUILDPLATFORM node:24.11-bookworm-slim AS web
WORKDIR /src
RUN corepack enable
COPY package.json pnpm-lock.yaml pnpm-workspace.yaml ./
RUN pnpm install --frozen-lockfile
COPY . .
RUN pnpm build

FROM --platform=$BUILDPLATFORM golang:1.26-bookworm AS binary
ARG TARGETOS TARGETARCH
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=web /src/public/build ./public/build
COPY --from=web /src/public/widget ./public/widget
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH \
    go build -trimpath -tags cloud -ldflags="-s -w" -o /out/dimmi ./cmd/dimmi
ADD --chmod=0444 https://truststore.pki.rds.amazonaws.com/global/global-bundle.pem /out/rds-global-bundle.pem

FROM binary AS source-binary

# Empty unless the build replaces it with --build-context prebuilt=<dir>.
FROM scratch AS prebuilt

FROM scratch AS prebuilt-binary
COPY --from=prebuilt dimmi /out/dimmi
ADD --chmod=0444 https://truststore.pki.rds.amazonaws.com/global/global-bundle.pem /out/rds-global-bundle.pem

FROM ${BINARY}-binary AS release-binary

FROM gcr.io/distroless/static-debian12:nonroot
ARG REVISION=unknown
ENV DIMMI_APP__RELEASE=$REVISION
LABEL org.opencontainers.image.source="https://github.com/dimmihq/dimmi" \
      org.opencontainers.image.revision=$REVISION
COPY --from=release-binary /out/rds-global-bundle.pem /etc/ssl/certs/rds-global-bundle.pem
COPY --from=release-binary /out/dimmi /dimmi
USER nonroot:nonroot
EXPOSE 8080 8443
ENTRYPOINT ["/dimmi"]
CMD ["serve"]
