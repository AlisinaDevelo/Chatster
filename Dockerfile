# syntax=docker/dockerfile:1

FROM node:20-bookworm-slim AS frontend-build
WORKDIR /src/frontend
COPY frontend/package.json frontend/package-lock.json ./
RUN npm ci
COPY frontend/ ./
RUN npm run build

FROM golang:1.22-bookworm AS backend-build
WORKDIR /src/backend
COPY backend/go.mod backend/go.sum ./
RUN go mod download
COPY backend/ ./
RUN CGO_ENABLED=1 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/chatster .

FROM debian:bookworm-slim
RUN apt-get update \
	&& apt-get install -y --no-install-recommends ca-certificates libsqlite3-0 \
	&& rm -rf /var/lib/apt/lists/*
RUN useradd --no-create-home --uid 65532 --user-group chatster
WORKDIR /app
COPY --from=backend-build /out/chatster /app/chatster
COPY --from=frontend-build /src/frontend/build /app/static
RUN mkdir -p /data && chown -R chatster:chatster /data /app/static
USER chatster:chatster
ENV CHATSTER_HTTP_ADDR=:8080
ENV CHATSTER_DB_PATH=/data/chatster.db
ENV CHATSTER_STATIC_DIR=/app/static
EXPOSE 8080
VOLUME ["/data"]
ENTRYPOINT ["/app/chatster"]
