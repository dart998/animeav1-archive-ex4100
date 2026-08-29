# animeav1-archive-ex4100

Archivador experimental para AnimeAV1 orientado a WD My Cloud EX4100 (ARMv7) y Portainer CE.

## Estado

Versión inicial de infraestructura: servicio web mínimo, healthcheck, persistencia y build ARMv7 sobre Debian Bullseye.

## Imagen

`ghcr.io/dart998/animeav1-archive-ex4100:latest`

## Portainer

Despliega `docker-compose.yml` desde el repositorio. Puerto web por defecto: `8090`.

Datos persistentes:

`/mnt/HD/HD_a2/Public/animeav1-archive`

## Seguridad funcional

La descarga de vídeo está desactivada por defecto (`DOWNLOAD_VIDEOS=false`). La primera iteración se limita a infraestructura, catálogo, metadatos, imágenes y detección de reproductores.
