#!/bin/sh
set -eu
IMAGE="${IMAGE:-ovelayos/animeav1-archive-ex4100}"
TAG="${1:-dev}"
docker buildx build --platform linux/arm/v7 --build-arg VERSION="$TAG" -t "$IMAGE:$TAG" --load .
echo "Built $IMAGE:$TAG for linux/arm/v7"
