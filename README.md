# AnimeAV1 Archive para WD EX4100

Aplicación local para navegar una copia de AnimeAV1 desde un WD My Cloud EX4100 (`linux/arm/v7`), reutilizar la biblioteca de vídeo existente del NAS y mantener una caché local de las páginas y recursos necesarios.

## Qué hace actualmente

- sirve el mirror de AnimeAV1 en `/`;
- usa una sesión autenticada de AnimeAV1 almacenada solo en backend;
- lee las listas personales de AnimeAV1 y prioriza el mirror en este orden: **Viendo → Planeado → Completado → resto**;
- guarda HTML y recursos del mirror bajo `/data/site`;
- usa `/data/seed` como fuente prioritaria para recursos estáticos conocidos;
- reescribe recursos del CDN a rutas locales `/_cdn/...` y reutiliza las copias existentes antes de volver a descargarlas;
- indexa recursivamente la biblioteca montada en `/library`;
- relaciona series locales mediante título, aliases, franquicia, temporada y patrones de episodios;
- prioriza nombres de episodio con el `mediaId` de AnimeAV1 cuando está disponible;
- reproduce directamente desde el NAS los formatos compatibles con el navegador;
- permite marcar episodios como vistos en AnimeAV1 desde la reproducción local;
- permite refrescar una ficha individual desde AnimeAV1;
- ofrece panel de administración en `/admin`, estado JSON y healthcheck;
- construye y publica automáticamente la imagen Docker para `linux/arm/v7`.

La aplicación nunca mueve, renombra ni elimina automáticamente archivos de la biblioteca local.

## Arquitectura

```text
Navegador
   |
   +--> /                  mirror local
   +--> /admin             administración
   +--> /api/status        estado
   +--> /healthz           healthcheck
           |
        Docker ARMv7
        |       |
        |       +--> /library   biblioteca existente
        +----------> /data      SQLite, mirror, seed y metadatos
```

## Despliegue en Portainer CE

El stack está definido en [`docker-compose.yml`](docker-compose.yml). La versión de imagen que debe desplegarse se mantiene únicamente en ese archivo y en el workflow de publicación; el README no fija un número de versión.

Configuración predeterminada:

```text
Puerto web:   8090
Datos:        /mnt/HD/HD_a2/Public/animeav1-archive
Biblioteca:   /mnt/HD/HD_a2/Public/Anime/Series
Zona horaria: Europe/Madrid
```

Después del despliegue:

1. abre `http://<EX4100>:8090/admin`;
2. guarda la cookie de sesión de AnimeAV1;
3. pulsa **Actualizar listas AV1**;
4. pulsa **Reindexar /library** cuando cambie la biblioteca;
5. pulsa **Descargar / actualizar sitio** para crear o refrescar el mirror;
6. abre `http://<EX4100>:8090/`.

## Sesión de AnimeAV1

La cookie se guarda en SQLite y solo se envía al dominio principal de AnimeAV1. No se incrusta en HTML o JavaScript del mirror ni se reenvía al CDN.

Si deja de funcionar la lectura de listas, sustituye la cookie desde `/admin`.

## Datos persistentes

```text
/data/
|-- db/archive.sqlite
|-- site/                    mirror y caché publicada
|-- seed/                    recursos estáticos de referencia
|-- metadata/
|-- images/
|-- logs/
`-- tmp/

/library/                    biblioteca de vídeo existente
```

Se indexan recursivamente `.mkv`, `.mp4`, `.avi`, `.webm`, `.m4v` y `.mov`.

## Matching de biblioteca local

El matching intenta usar, por orden de confianza:

1. título o alias exacto;
2. título base/franquicia con temporada relacionada;
3. alias significativo incluido en el nombre de una carpeta;
4. para episodios, patrón fuerte `<mediaId>_<episodio>_...`;
5. patrones explícitos como `S01E05`, `Ep05` o equivalentes;
6. número de episodio aislado como último recurso.

Las temporadas localizadas en subcarpetas como `Temporada 2`, `Season 2` o `S02` se priorizan para evitar mezclar episodios de temporadas distintas.

## Mirror y recursos

La prioridad de recursos es:

```text
seed disponible
  -> copia local del mirror
  -> descarga del origen/CDN
```

El panel muestra el total real de archivos existentes bajo `/data/site`, los guardados durante la sincronización actual y un árbol desplegable organizado por carpetas. El log del mirror se presenta con aspecto de terminal y muestra descargas guardadas y errores de la sincronización.

Los archivos de vídeo/audio remotos no forman parte del mirror del sitio. La reproducción local utiliza directamente los archivos de `/library` cuando existe una coincidencia.

## Variables principales

| Variable | Predeterminado | Uso |
| --- | --- | --- |
| `WEB_PORT` | `8090` | Puerto publicado |
| `TZ` | `Europe/Madrid` | Zona horaria |
| `ANIMEAV1_BASE_URL` | `https://animeav1.com` | Origen del mirror |
| `CRAWLER_ENABLED` | `true` | Tarea periódica del crawler |
| `CRAWLER_INTERVAL` | `30m` | Intervalo del crawler |
| `CRAWLER_BATCH_SIZE` | `5` | Series por lote |
| `CRAWLER_CONCURRENCY` | `1` | Paralelismo del crawler |
| `LOG_LEVEL` | `info` | Nivel de log |

Consulta `docker-compose.yml` para la lista completa y los valores usados en el stack.

## Build

Requiere Docker con Buildx:

```sh
./build.sh
```

Publicación manual:

```sh
./release.sh <version>
```

El flujo normal usa GitHub Actions para construir y publicar la imagen ARMv7. Portainer puede detectar el cambio del `docker-compose.yml` mediante GitOps polling.

## Seguridad

- la cookie de AnimeAV1 permanece en backend;
- no se envía al CDN;
- se bloquean destinos publicitarios conocidos y EasyList complementa ese filtrado;
- no se implementan bypass de DRM, tokens ni controles de acceso de proveedores;
- Portainer no necesita exponerse a Internet para actualizar el stack.

## Pendiente

- permitir marcar y desmarcar favoritos desde las fichas, reproduciendo la acción real de AnimeAV1;
- mejorar el fallback entre reproductor local y reproductores online legítimos;
- seguir afinando la clasificación de OVAs, películas y spin-offs cuando una franquicia comparte carpeta raíz.
