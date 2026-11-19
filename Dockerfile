# Base images are pinned by digest (Dependabot keeps them up to date).

# --- Build: static binary (pure Go SQLite driver, no CGO) ---
# Runs on the build machine's platform and cross-compiles for the target, so
# multi-arch builds need no emulation.
FROM --platform=$BUILDPLATFORM golang:1.27.4-alpine@sha256:70bd7ddea8b6cbe5d6943cde279d6b67feb10cc924fb9100d1d8757c1575b280 AS build
WORKDIR /src
ENV CGO_ENABLED=0 GOTOOLCHAIN=local
COPY go.mod go.sum ./
RUN go mod download
COPY . .
# Pages are templ components: generate their Go code (not committed).
RUN go tool templ generate
ARG TARGETOS TARGETARCH TARGETVARIANT
RUN GOOS=$TARGETOS GOARCH=$TARGETARCH GOARM=${TARGETVARIANT#v} \
    go build -trimpath -ldflags '-s -w' -o /out/benevole ./cmd/benevole \
 && mkdir -p /out/data

# --- Run: distroless, non-root (uid 65532), CA certs and tzdata included ---
FROM gcr.io/distroless/static-debian13:nonroot@sha256:aafe993665bf6200db38a02c11a9351e782a84b6a45e7f33b6695638c88e3931
COPY --from=build /out/benevole /usr/local/bin/benevole
# Empty data dir owned by the runtime user, so a named volume mounted here
# is writable out of the box.
COPY --from=build --chown=65532:65532 /out/data /data

ENV BENEVOLE_ADDR=:8080 \
    BENEVOLE_DB=/data/benevole.db \
    TZ=Europe/Paris
VOLUME ["/data"]
EXPOSE 8080
USER nonroot:nonroot

HEALTHCHECK --interval=30s --timeout=5s --start-period=5s --retries=3 \
  CMD ["/usr/local/bin/benevole", "-healthcheck"]

ENTRYPOINT ["/usr/local/bin/benevole"]
