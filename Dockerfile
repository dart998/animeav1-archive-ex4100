FROM --platform=$BUILDPLATFORM golang:1.23-bullseye AS build
ARG TARGETOS
ARG TARGETARCH
ARG TARGETVARIANT
ARG VERSION=dev
ARG COMMIT_SHA=unknown
WORKDIR /src
COPY go.mod ./
RUN go mod download
COPY . .
RUN go mod tidy
RUN CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH:-arm} GOARM=7 go build -trimpath -ldflags="-s -w -X main.version=${VERSION} -X main.commitSHA=${COMMIT_SHA}" -o /out/animeav1-archive ./cmd/server

FROM debian:bullseye-slim
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates tzdata && rm -rf /var/lib/apt/lists/*
WORKDIR /app
COPY --from=build /out/animeav1-archive /usr/local/bin/animeav1-archive
COPY web /app/web
RUN mkdir -p /data/db /data/metadata /data/images /data/site /data/logs /data/tmp
EXPOSE 8080
VOLUME ["/data"]
ENTRYPOINT ["/usr/local/bin/animeav1-archive"]
