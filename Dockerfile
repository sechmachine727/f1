# syntax=docker/dockerfile:1.7
FROM node:24-alpine AS web
WORKDIR /src/race_engineer_hub
COPY race_engineer_hub/package.json race_engineer_hub/package-lock.json ./
RUN npm ci
COPY race_engineer_hub/ ./
RUN npm run build

FROM golang:1.27.1-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY server/ ./server/
COPY --from=web /src/race_engineer_hub/dist ./server/internal/web/dist
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/f1tele ./server/cmd/f1tele
RUN mkdir -p /out/data && chown 65532:65532 /out/data

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/f1tele /f1tele
COPY --from=build --chown=65532:65532 /out/data /data
USER nonroot:nonroot
VOLUME ["/data"]
EXPOSE 8081/tcp 20777/udp
ENTRYPOINT ["/f1tele","serve","--data-dir","/data"]
