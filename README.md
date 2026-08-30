# AnimeAV1 Archive para WD EX4100

Mirror local y base de archivado de AnimeAV1 para un WD My Cloud EX4100 (`linux/arm/v7`). La aplicación se ejecuta en Docker, se administra desde el navegador y reutiliza la biblioteca de vídeo que ya existe en el NAS.

La versión preparada por el repositorio es la **0.6.0**.

## Estado actual

El proyecto ya no es la prueba de concepto limitada a `mao`. Actualmente incluye:

- mirror híbrido de AnimeAV1 servido en la raíz `/`;
- descarga inicial del sitio y caché bajo demanda de las páginas visitadas;
- caché local de HTML, CSS, JavaScript, imágenes y fuentes;
- sanitización de publicidad, popups, redirecciones y recursos de terceros;
- bloqueo adicional basado en EasyList;
- sesión autenticada de AnimeAV1, almacenada en SQLite y enviada únicamente a `animeav1.com`;
- lectura de las listas personales de AnimeAV1;
- alcance definido por series en estado **Viendo** o **Completado**;
- inventario de la biblioteca montada en `/library`;
- comparación por título y alias entre las listas de AnimeAV1 y las carpetas locales;
- sugerencias de nombre cuando una carpeta coincide con una serie pero el nombre difiere;
- integración opcional con la lista pública de MyAnimeList;
- panel de administración separado en `/admin`;
- versión y enlace al commit exacto visibles en el panel;
- API de estado y healthcheck;
- imagen Docker ARMv7 construida y publicada automáticamente;
- despliegue automático en Portainer CE mediante webhook HTTPS y tags versionados.

La descarga automática de episodios todavía no constituye el flujo principal terminado. El código puede detectar proveedores y contiene una ruta de archivado con `ffmpeg`, pero la conexión completa **lista AnimeAV1 → serie → episodios → reutilización local → descarga selectiva** sigue pendiente. Por seguridad, la descarga está desactivada por defecto.

## Arquitectura

```text
Navegador en la red local
        |
        +--> /             mirror de AnimeAV1
        +--> /admin        administración
        +--> /api/status   estado JSON
        +--> /healthz      healthcheck
                |
        contenedor ARMv7 en el EX4100
          |             |
          |             +--> /library (biblioteca existente, lectura/reutilización)
          +--> /data (SQLite, caché, metadatos y estado)
```

El backend trabaja con las respuestas originales antes de sanitizarlas. La limpieza se aplica a la copia que se publica en el mirror, de modo que la futura extracción autorizada de fuentes no dependa del HTML ya modificado.

## Despliegue en Portainer

El archivo [`docker-compose.yml`](docker-compose.yml) está preparado para desplegarse como stack desde el repositorio.

Configuración predeterminada:

```text
Imagen:       ovelayos/animeav1-archive-ex4100:0.5.2
Puerto web:   8090
Datos:        /mnt/HD/HD_a2/Public/animeav1-archive
Biblioteca:   /mnt/HD/HD_a2/Public/Anime/Series
Zona horaria: Europe/Madrid
```

La biblioteca se monta dentro del contenedor como `/library`. El directorio de datos se monta como `/data`.

Después del despliegue:

1. Abre `http://<EX4100>:8090/admin`.
2. Guarda la cookie de sesión de AnimeAV1.
3. Pulsa **Actualizar listas AV1**.
4. Pulsa **Reindexar /library** para actualizar el inventario local cuando sea necesario.
5. Pulsa **Descargar / actualizar sitio** para crear o refrescar el mirror.
6. Abre `http://<EX4100>:8090/` para navegar por la copia local.

## Sesión de AnimeAV1

La cookie se introduce en `/admin` con un formato como:

```text
session=...; PARAGLIDE_LOCALE=es
```

Se guarda en la base SQLite, no vuelve a mostrarse en el formulario y solo se añade a solicitudes dirigidas al dominio principal de AnimeAV1. No se envía al CDN ni se incrusta en el HTML o JavaScript del mirror.

Si las listas dejan de actualizarse, la sesión probablemente haya caducado y debe sustituirse desde el panel.

## Variables de entorno

| Variable | Valor predeterminado | Uso |
| --- | --- | --- |
| `ARCHIVE_IMAGE` | `ovelayos/animeav1-archive-ex4100:0.5.2` | Imagen exacta que ejecutará el stack |
| `WEB_PORT` | `8090` | Puerto publicado en el NAS |
| `TZ` | `Europe/Madrid` | Zona horaria |
| `ANIMEAV1_BASE_URL` | `https://animeav1.com` | Origen del mirror y del crawler |
| `CRAWLER_ENABLED` | `true` | Activa la tarea periódica del crawler |
| `CRAWLER_INTERVAL` | `30m` | Intervalo de la tarea periódica |
| `CRAWLER_BATCH_SIZE` | `5` | Series procesadas en cada ciclo incremental |
| `PROVIDER_ORDER` | `hls,upnshare,mega,mp4upload` | Prioridad de proveedores detectados |
| `PROVIDER_FALLBACK` | `true` | Prueba el siguiente proveedor si uno falla |
| `DOWNLOAD_VIDEOS` | `false` | Habilita la ruta de archivado de vídeo |
| `VIDEO_DOWNLOAD_AUTHORIZED` | `false` | Confirmación adicional de autorización |
| `LOG_LEVEL` | `info` | Nivel de log configurado para el contenedor |

`DOWNLOAD_VIDEOS` y `VIDEO_DOWNLOAD_AUTHORIZED` deben estar ambos a `true` para que el código permita archivar una fuente. No se deben activar hasta completar y validar el flujo de selección de contenido y disponer de autorización para guardar la fuente correspondiente.

## Datos persistentes

```text
/data/
|-- db/archive.sqlite       configuración, listas, inventario y estado
|-- site/                   caché publicada del mirror
|-- metadata/<serie>/       metadatos JSON de episodios analizados
|-- images/
|-- logs/
`-- tmp/

/library/                   biblioteca de series ya existente
```

La aplicación indexa como vídeo los archivos `.mkv`, `.mp4`, `.avi`, `.webm`, `.m4v` y `.mov`. El escaneo se realiza al arrancar, manualmente desde `/admin` y periódicamente cada seis horas.

## Mirror y seguridad

El mirror permite el dominio principal de AnimeAV1 y su CDN, pero bloquea recursos publicitarios y destinos externos conocidos. También neutraliza mecanismos habituales de popup o navegación no deseada en la copia servida al navegador.

Los archivos de vídeo y audio no forman parte de la descarga del sitio. Entre las rutas y extensiones excluidas están `m3u8`, `mp4`, `mkv`, `webm`, `ts`, `mp3` y `/api/video`.

El mirror no implementa bypass de DRM, evasión de tokens ni mecanismos para saltarse controles de acceso de proveedores.

## MyAnimeList

MAL permanece como integración opcional. Puede guardar un usuario público, leer su lista **Watching** y compararla con la biblioteca local, pero ya no define qué series pertenecen al alcance del proyecto. Ese alcance lo determinan las listas autenticadas de AnimeAV1.

## Build local

Requiere Docker con soporte Buildx:

```sh
./build.sh
```

Para publicar manualmente una versión:

```sh
./release.sh 0.6.0
```

El flujo normal de publicación lo realiza GitHub Actions para `linux/arm/v7` e inyecta en el binario la versión y el SHA del commit.

## Despliegue automático

Cada cambio aplicable en `main` ejecuta este flujo:

```text
push a main
  -> build ARMv7
  -> publicación de :<versión> y :latest
  -> actualización del tag versionado en docker-compose.yml
  -> POST al webhook HTTPS de Portainer
  -> redeploy del stack
```

El stack usa un tag versionado e inmutable, no `latest`. Portainer Community Edition no garantiza un nuevo pull de `latest` al ejecutar un webhook porque la opción **Re-pull image** pertenece a Business Edition. Al cambiar el compose a un tag nuevo, Docker debe obtener esa imagen concreta y el rollback consiste en volver al tag anterior.

El workflow necesita estos secretos del repositorio:

- `DOCKERHUB_USERNAME`
- `DOCKERHUB_TOKEN`
- `PORTAINER_WEBHOOK_URL`

Los commits automáticos que solo actualizan `docker-compose.yml` no vuelven a lanzar el build, evitando un bucle. La concurrencia también cancela builds anteriores cuando llega un commit más reciente.

El webhook público debe permanecer detrás del proxy HTTPS del EX4100. Solo se expone la ruta exacta `/api/stacks/webhooks/<id>` mediante `POST`; el puerto `9000` de Portainer no se publica directamente en Internet.

## Trabajo pendiente

- mejorar el reconocimiento de episodios locales por nombre de archivo;
- reutilizar archivos locales a nivel de episodio, no solo a nivel de carpeta;
- implementar controles para descargar o redescargar una serie o un episodio;
- mostrar el progreso real de contenido local frente al total disponible;
- completar pruebas automatizadas del parser, matching, sanitización y rutas web;
- definir el ciclo de renovación del certificado del proxy del webhook.

## Uso responsable

Utiliza el proyecto únicamente con contenido y fuentes que tengas derecho a consultar o archivar. La doble confirmación de descarga es una barrera intencionada y no sustituye la autorización correspondiente.
