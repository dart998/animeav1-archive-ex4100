#!/bin/sh
set -eu
VERSION="${1:?usage: ./release.sh VERSION}"
IMAGE="${IMAGE:-ovelayos/animeav1-archive-ex4100}"
git pull --ff-only
docker buildx build --platform linux/arm/v7 --build-arg VERSION="$VERSION" -t "$IMAGE:$VERSION" -t "$IMAGE:latest" --push .
echo "Published $IMAGE:$VERSION and $IMAGE:latest"
