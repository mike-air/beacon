# Beacon web — multi-stage build.
#
# Build context is the REPOSITORY ROOT, not this directory, because the app
# depends on ../sdk as a file: dependency:
#
#     docker build -f web/Dockerfile -t beacon-web:local .
#
# Stage one builds the bundle, which also runs the guards: the token generator
# (which refuses to emit a palette that fails contrast), tsc, and the
# performance budget. A container that builds is therefore a container whose
# colours are readable and whose entry bundle is under 200KB gzipped. That is
# deliberate — a guard you can skip is a guard.
#
# Stage two is nginx serving static files, unprivileged, with the API base
# injected at START-UP rather than baked in. See deploy/docker-entrypoint.sh.

# ---- build stage -----------------------------------------------------------
FROM node:24-alpine AS build

WORKDIR /app

# The SDK is consumed as TypeScript source (its package.json points main at
# src/index.ts) and has no runtime dependencies of its own, so it only needs
# to be PRESENT before web's npm ci resolves the file: link.
COPY sdk/ ./sdk/

# Manifests first, so a code-only change does not re-install.
COPY web/package.json web/package-lock.json ./web/
WORKDIR /app/web
RUN npm ci --ignore-scripts

COPY web/ ./

# gen:tokens (contrast guard) -> tsc --noEmit -> vite build -> budget check.
RUN npm run build

# ---- final stage -----------------------------------------------------------
# The unprivileged image listens on 8080 as uid 101 with no root process at
# all, which matches the API's distroless:nonroot posture. The stock nginx
# image runs its master as root; nothing here needs it.
FROM nginxinc/nginx-unprivileged:1.29-alpine

# nginx.conf is a TEMPLATE: the base image runs envsubst over
# /etc/nginx/templates at start-up, which is how ${BEACON_API_BASE} reaches
# the CSP. docker-entrypoint.sh writes the same value into config.js.
COPY web/deploy/nginx.conf /etc/nginx/templates/default.conf.template
# Also a template: the CSP needs ${BEACON_API_BASE} substituted into it, and
# it lands in conf.d/ where the location blocks include it.
COPY web/deploy/security.conf /etc/nginx/templates/beacon-security.conf.template
COPY web/deploy/docker-entrypoint.sh /docker-entrypoint.d/99-beacon-config.sh
COPY --from=build /app/web/dist /usr/share/nginx/html

EXPOSE 8080

# The base image's entrypoint runs /docker-entrypoint.d/* and then nginx.
