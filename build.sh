#!/bin/sh
set -eu

IMAGE="${IMAGE:-ghcr.io/dart998/animeav1-archive-ex4100}"
TAG="${TAG:-dev}"

docker buildx build \
  --platform linux/arm/v7 \
  --build-arg VERSION="$TAG" \
  -t "$IMAGE:$TAG" \
  --load \
  .

echo "Built $IMAGE:$TAG"
