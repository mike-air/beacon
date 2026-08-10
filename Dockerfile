# Beacon — multi-stage build.
#
# Course mapping: Chapter 41 — Dockerfile. A fat build stage compiles a static,
# CGO-free binary; a tiny distroless final stage ships only that binary, runs as
# a non-root user, and carries CA certificates for outbound TLS (SMTP, S3, the
# webhook deliverer). No shell, no package manager, minimal attack surface.

# ---- build stage -----------------------------------------------------------
FROM golang:1.23 AS build

WORKDIR /src

# Cache deps separately from source so a code-only change doesn't re-download.
COPY go.mod go.sum ./
RUN go mod download

COPY . .

# CGO disabled → statically linked binaries that run on scratch/distroless.
# -trimpath drops local filesystem paths; -ldflags "-s -w" strips debug info.
# Both binaries share the same code, so we build them in the one stage.
ENV CGO_ENABLED=0 GOOS=linux
RUN go build -trimpath -ldflags="-s -w" -o /out/beacon-api    ./cmd/beacon-api && \
    go build -trimpath -ldflags="-s -w" -o /out/beacon-worker ./cmd/beacon-worker

# ---- final stage -----------------------------------------------------------
# distroless/static carries CA certs and /etc/passwd with a "nonroot" user, but
# no shell and no libc — exactly what a static Go binary needs and nothing more.
# One image ships both binaries; the API is the default. Run the worker by
# overriding the entrypoint:
#   docker run --entrypoint /usr/local/bin/beacon-worker beacon-api:local
FROM gcr.io/distroless/static:nonroot

# Run unprivileged. The nonroot user (uid 65532) ships with the base image.
USER nonroot:nonroot

COPY --from=build /out/beacon-api    /usr/local/bin/beacon-api
COPY --from=build /out/beacon-worker /usr/local/bin/beacon-worker

EXPOSE 8080

ENTRYPOINT ["/usr/local/bin/beacon-api"]
