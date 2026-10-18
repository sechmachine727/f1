# syntax=docker/dockerfile:1

# ---- Stage 1: build the Race Engineer Hub dashboard ----
FROM node:22-slim AS ui
WORKDIR /ui
COPY race_engineer_hub/package.json race_engineer_hub/package-lock.json ./
RUN npm ci --no-audit --no-fund
COPY race_engineer_hub/ ./
# Empty by default: the bundle derives ws://<page host>:8765 at runtime.
ARG VITE_WS_URL=
ENV VITE_WS_URL=${VITE_WS_URL}
RUN npm run build

# ---- Stage 2: telemetry bridge + static dashboard ----
FROM python:3.12-slim

ENV PYTHONPATH=/app \
    PYTHONUNBUFFERED=1 \
    PIP_DISABLE_PIP_VERSION_CHECK=1

WORKDIR /app

COPY requirements.txt ./
RUN pip install --no-cache-dir -r requirements.txt

COPY . .
COPY --from=ui /ui/dist ./race_engineer_hub/dist

RUN chmod +x /app/docker/entrypoint.sh

EXPOSE 20777/udp 8765 8081

ENTRYPOINT ["/app/docker/entrypoint.sh"]
