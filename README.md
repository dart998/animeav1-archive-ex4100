# animeav1-archive-ex4100

Archivador/mirror local experimental para AnimeAV1, diseñado para WD My Cloud EX4100 (`linux/arm/v7`) con Docker y Portainer CE.

## POC actual

Objetivo inicial: `https://animeav1.com/media/mao`.

La aplicación:

- rastrea la ficha y descubre episodios;
- almacena anime, episodios, ejecuciones y fuentes en SQLite;
- registra todos los proveedores detectados;
- prioriza `HLS -> UPNShare -> Mega -> MP4Upload`;
- valida únicamente URLs que estén expuestas en el HTML sin eludir controles de acceso;
- guarda JSON por episodio;
- muestra estado y resumen en una UI local;
- puede archivar una fuente autorizada mediante ffmpeg y deduplicar por SHA-256.

No implementa bypass de DRM, resolución de protecciones, evasión de tokens ni técnicas para saltarse controles de los proveedores.

## Imagen

`ovelayos/animeav1-archive-ex4100:latest`

Runtime Debian Bullseye; build objetivo `linux/arm/v7`.

## Build local

```sh
./build.sh
```

## Publicar release

```sh
./release.sh 0.1.0
```

El script publica el tag indicado y `latest` en Docker Hub. Debes tener `docker login` hecho.

## Portainer

Usa `docker-compose.yml` desde Repository.

Puerto web por defecto: `8090`.

Persistencia:

`/mnt/HD/HD_a2/Public/animeav1-archive:/data`

Variables relevantes:

- `CRAWLER_ENABLED=true`
- `CRAWLER_INTERVAL=12h`
- `CRAWLER_TARGETS=mao`
- `PROVIDER_ORDER=hls,upnshare,mega,mp4upload`
- `PROVIDER_FALLBACK=true`
- `DOWNLOAD_VIDEOS=false`
- `VIDEO_DOWNLOAD_AUTHORIZED=false`

Para guardar vídeo deben estar a `true` tanto `DOWNLOAD_VIDEOS` como `VIDEO_DOWNLOAD_AUTHORIZED`. La segunda variable es una confirmación explícita de que tienes autorización para archivar esa fuente.

## Datos

```text
/data/
├── db/archive.sqlite
├── metadata/<slug>/<episodio>.json
├── images/
├── videos/<slug>/<episodio>.mp4
├── site/
├── logs/
└── tmp/
```

## UI

- `/` resumen del crawler y animes archivados
- `/healthz` healthcheck
- `/api/status` estado JSON del crawler

## Limitación conocida de la POC

AnimeAV1 puede mostrar el nombre de un proveedor sin exponer directamente la URL final del medio en el HTML inicial. En ese caso se registra el proveedor como detectado, pero no se intenta resolverlo mediante mecanismos de evasión. Esto permite identificar qué proveedores están disponibles y evolucionar el extractor usando únicamente flujos autorizados.
