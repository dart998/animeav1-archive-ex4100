#!/bin/sh
set -eu

IMAGE="${IMAGE:-ghcr.io/dart998/animeav1-archive-ex4100}"
TAG="${1:-0.1.0}"

git pull --ff-only

docker buildx build \
  --platform linux/arm/v7 \
  --build-arg VERSION="$TAG" \
  -t "$IMAGE:$TAG" \
  -t "$IMAGE:latest" \
  --push \
  .

echo "Published $IMAGE:$TAG and $IMAGE:latest"
