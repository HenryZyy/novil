# syntax=docker/dockerfile:1

FROM node:24-alpine AS frontend

WORKDIR /app

RUN corepack enable

COPY apps/frontend/package.json apps/frontend/pnpm-lock.yaml ./
RUN pnpm install --frozen-lockfile

COPY apps/frontend ./
RUN pnpm build

FROM golang:1.27-alpine AS backend

WORKDIR /src

COPY apps/backend/go.mod apps/backend/go.sum ./
RUN go mod download

COPY apps/backend ./
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/novil ./cmd/novil \
  && mkdir -p /out/config

FROM gcr.io/distroless/static-debian12:nonroot AS runtime

ENV NOVIL_ADDR=:8080 \
    NOVIL_DATA_DIR=/config \
    NOVIL_WEB_DIR=/web

# The base image's /home/nonroot is only open to its own user, but compose
# runs the container as another one, which then can't even check for a .env
# there. Settings come from the environment (compose env_file) instead.
WORKDIR /

COPY --from=backend /out/novil /novil
COPY --from=frontend /app/dist /web
COPY --from=backend --chown=65532:65532 /out/config /config

VOLUME ["/config"]
EXPOSE 8080

ENTRYPOINT ["/novil"]
