# syntax=docker/dockerfile:1

FROM golang:1.22-bullseye AS builder
WORKDIR /src
COPY go.mod ./
COPY cmd ./cmd
ARG VERSION=dev
ARG TARGETOS
ARG TARGETARCH
ARG TARGETVARIANT
RUN CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH:-arm} GOARM=7 \
    go build -trimpath -ldflags="-s -w -X main.version=${VERSION}" -o /out/animeav1-archive ./cmd/server

FROM debian:bullseye-slim
RUN apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates tzdata \
    && rm -rf /var/lib/apt/lists/*
WORKDIR /app
COPY --from=builder /out/animeav1-archive /usr/local/bin/animeav1-archive
ENV TZ=Europe/Madrid \
    PORT=8080 \
    ARCHIVE_DATA_DIR=/data
VOLUME ["/data"]
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/animeav1-archive"]
