# The image a release publishes, from binaries the release workflow already
# built, one directory per edition and architecture:
#
#   <edition>_linux_<arch>/dimmi    edition is self-host or cloud
#
#   docker buildx build -f deploy/release/Dockerfile --platform linux/amd64,linux/arm64 \
#     --build-arg EDITION=self-host --build-arg VERSION=0.1.0 dist
#
# Nothing runs inside the build, so every architecture builds without
# emulation. The RDS certificate bundle lets the cloud edition verify RDS.

FROM gcr.io/distroless/static-debian12:nonroot
ARG TARGETARCH EDITION=self-host VERSION=dev REVISION=unknown
ENV DIMMI_APP__RELEASE=$VERSION
LABEL org.opencontainers.image.source="https://github.com/dimmihq/dimmi" \
      org.opencontainers.image.version=$VERSION \
      org.opencontainers.image.revision=$REVISION
ADD --chmod=0444 https://truststore.pki.rds.amazonaws.com/global/global-bundle.pem /etc/ssl/certs/rds-global-bundle.pem
COPY --chmod=0555 ${EDITION}_linux_${TARGETARCH}/dimmi /dimmi
USER nonroot:nonroot
EXPOSE 8080 8443
ENTRYPOINT ["/dimmi"]
CMD ["serve"]
