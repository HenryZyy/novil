# syntax=docker/dockerfile:1

FROM golang:1.27-alpine AS build

WORKDIR /src

COPY apps/backend/go.mod apps/backend/go.sum ./
RUN go mod download

COPY apps/backend ./
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/novil ./cmd/novil \
  && mkdir -p /out/config

FROM gcr.io/distroless/static-debian12:nonroot AS runtime

ENV NOVIL_ADDR=:8080 \
    NOVIL_DATA_DIR=/config

COPY --from=build /out/novil /novil
# Lets the default nonroot user write to /config when it is a named volume.
COPY --from=build --chown=65532:65532 /out/config /config

VOLUME ["/config"]
EXPOSE 8080

ENTRYPOINT ["/novil"]
