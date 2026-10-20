# Base images pin the verified multi-arch manifest digests resolved on 2026-10-20.
FROM node:24-alpine@sha256:44fce696f38ba8c987c4db89954051000bbd05ffd8d8659ba15ed4a8602e937f AS web
WORKDIR /src/race_engineer_hub
COPY race_engineer_hub/package.json race_engineer_hub/package-lock.json ./
RUN npm ci
COPY race_engineer_hub/ ./
RUN npm run build

FROM golang:1.27.1-alpine@sha256:8a5910f31396cd4d89662f56c68b3ae31d374308270a1c3bd96672ee5ed43414 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY server/ ./server/
COPY --from=web /src/race_engineer_hub/dist ./server/internal/web/dist
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/f1tele ./server/cmd/f1tele
RUN mkdir -p /out/data && chown 65532:65532 /out/data

FROM gcr.io/distroless/static-debian12:nonroot@sha256:afa5c872c891853ca7fcf1f12c3edb23f7eeef36189728842dd51042ff57f7ab
ARG VERSION=dev
ARG REVISION=unknown
LABEL org.opencontainers.image.title="f1tele" \
      org.opencontainers.image.description="F1 25 telemetry recorder and Race Engineer Hub" \
      org.opencontainers.image.source="https://github.com/sechmachine727/f1" \
      org.opencontainers.image.version="$VERSION" \
      org.opencontainers.image.revision="$REVISION" \
      org.opencontainers.image.licenses="Apache-2.0"
COPY --from=build /out/f1tele /f1tele
COPY --from=build --chown=65532:65532 /out/data /data
USER nonroot:nonroot
VOLUME ["/data"]
EXPOSE 8081/tcp 20777/udp
STOPSIGNAL SIGTERM
ENTRYPOINT ["/f1tele","serve","--data-dir","/data"]

