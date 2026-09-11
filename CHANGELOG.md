# Changelog

Todos los cambios relevantes del proyecto se registran en este archivo a partir de la versión 0.6.6.

## [0.6.12] - 2026-09-11

### Corregido
- Los episodios locales en formatos que el navegador no reproduce directamente, como MKV/AVI, dejan de abrir un `<video>` roto y pasan automáticamente al reproductor remoto. El archivo local sigue contando como existente y no se vuelve a descargar.
- El reproductor remoto oculta siempre HLS/Zilla y UPNShare/uns.bio, pero conserva todos los demás proveedores publicados por AnimeAV1 para cada episodio, incluidos Voe, Byse, Mega, MP4Upload y proveedores futuros compatibles.

### Añadido
- Vuelve el botón **Descargar todos** en la ficha de la serie.
- La descarga recorre episodio por episodio, omite cualquier episodio ya presente en `/library`, busca un enlace Mega en la página remota del episodio y descarga secuencialmente al NAS.
- Las descargas Mega se resuelven desde el enlace público `mega.nz/embed/...#clave`, se descifran durante la escritura y se guardan primero como `.part` antes del renombrado final.
- Solo se ejecuta una descarga de serie a la vez para reducir carga en el EX4100 y al finalizar se reindexa `/library`.

## [0.6.11] - 2026-09-11

### Corregido
- Las acciones actuales de AnimeAV1 vuelven a funcionar desde el mirror: búsqueda en vivo (`POST /api/search`), alta/cambio/eliminación de lista (`POST /api/user/library`), favorito (`POST /api/user/library/favorite`) y edición completa desde Mis Listas (`POST /cuenta/listas?/library`). Las peticiones se proxyan desde backend conservando la cookie únicamente en el servidor.
- Tras una modificación de listas o favorito se refresca en segundo plano la caché `animeav1_library_json`, evitando que `/admin` quede desfasado.
- La detección de episodios locales acepta también nombres históricos del tipo `<id-antiguo>_<episodio>_...`, como `131_3_RhWz.mp4`, aunque el `mediaId` actual de AnimeAV1 sea distinto.
- Si una serie no está en la caché personal de AnimeAV1, la reproducción local puede usar el título indexado por el crawler como respaldo en lugar de abortar inmediatamente.
- El fallback online devuelve todos los proveedores detectados; la interfaz permite pasar al siguiente proveedor o abrirlo fuera si uno bloquea el iframe mediante `X-Frame-Options`/`frame-ancestors`.
- Los cuatro logos oficiales de AnimeAV1 se sirven siempre desde la imagen Docker en las rutas originales `/img/logo.svg`, `/img/logo-dark.svg`, `/img/logo-ft.svg` y `/img/logo-ft-dark.svg`, respetando los temas claro y oscuro.

### Retirado
- Se elimina de la interfaz el botón **Descargar todos** y se retira toda la implementación específica de Transfer.it. Los antiguos endpoints de descarga quedan desactivados con HTTP 410 para evitar usos accidentales mientras se diseña un sistema multi-proveedor.

### Diagnóstico
- Los logos locales, vídeos locales y fallback de CDN añaden `X-AnimeAV1-Source` para facilitar la identificación de la fuente desde DevTools.

## [0.6.10] - 2026-09-11

### Corregido
- Las imágenes `covers`, `screenshots` y `backdrops` mantienen prioridad local, pero si el backend del EX4100 recibe un error del CDN, `/_cdn/...` redirige al CDN real para que el navegador pueda cargar la imagen directamente. La CSP permite ese fallback solo para imágenes de `cdn.animeav1.com`.
- La reproducción local vuelve a decidirse por episodio. Si el capítulo concreto existe en `/library`, se usa el vídeo local; si no existe, se recupera un reproductor online legítimo de la ficha original en lugar de dejar la línea «Reproductor externo no disponible en la copia local».
- Se elimina el botón/texto redundante «Marcar episodio como visto / Visto en AnimeAV1» debajo del vídeo. El estado se sigue gestionando con el botón del ojo de la interfaz original.
- El panel `/admin` deja de recorrer `/data/site` cada cinco segundos. El total de recursos se guarda en SQLite (`settings`) y el árbol completo solo se calcula al abrir explícitamente Recursos guardados o Ver logs.

### Añadido
- Botón **Descargar todos** y primera implementación basada en Transfer.it. Retirada posteriormente en 0.6.11 al comprobarse que el proveedor cambia según la serie.
- Endpoint para recuperar el reproductor online preferido desde la ficha original cuando no hay copia local del episodio.

## [0.6.9] - 2026-09-10

### Añadido
- Matching local por franquicia y alias significativo para casos como DanMachi.
- Resolución de temporada a partir de títulos/aliases y subcarpetas como `Temporada 2`, `Season 2` o `S02`, evitando mezclar episodios entre temporadas.
- Contador real de recursos existentes bajo `/data/site` y contador separado de archivos guardados durante la sincronización actual.
- Popup de recursos guardados con árbol desplegable organizado por carpetas, nombre y tamaño de archivo.
- Vista de logs del mirror con aspecto de terminal, incluyendo recursos guardados y errores recientes.
- Endpoint `/api/mirror/resources` para consultar inventario y log de recursos del mirror.

### Cambiado
- El panel `/admin` usa el matcher multi-carpeta/franquicia en lugar de depender exclusivamente de coincidencia exacta.
- Se mejoran las cabeceras usadas al solicitar imágenes a `cdn.animeav1.com`, con User-Agent de navegador, `Accept` de imagen y cabeceras `Sec-Fetch-*`, sin reenviar la cookie de AnimeAV1.
- El README se reescribe para reflejar el funcionamiento actual, sin referencias a la prueba inicial ni números de versión fijos.
- El contador de elementos sanitizados se sustituye por **Guardados en esta sincro**.
- Se eliminan los tests unitarios y el paso `go test ./...` del workflow para reducir el tiempo de generación de versiones. La imagen ARMv7 sigue compilándose en GitHub Actions.

## [0.6.8] - 2026-09-09

### Añadido
- Prioridad efectiva `seed > caché > origen` para recursos estáticos conocidos.
- Detección local multi-carpeta por serie para agrupar temporadas, Parts, OVAs/OADs/especiales relacionados sin mover ni renombrar archivos.
- Coincidencia fuerte de episodios descargados de AnimeAV1 mediante `<mediaId>_<episodio>_...`, por delante de patrones genéricos.

## [0.6.7] - 2026-09-09
- Reescritura post-hidratación de imágenes del CDN a `/_cdn/...` y reinserción del botón de refresco.

## [0.6.6] - 2026-09-09
- Botón de actualización individual en las fichas y endpoint interno de refresco.

## [0.6.5] - 2026-09-09
- Reproducción de episodios locales y sincronización de episodios vistos con AnimeAV1.

## [0.6.4] - 2026-09-08
- Mirror incremental y reanudable.

## [0.6.3] - 2026-09-06
- Fallback local desde `/data/seed` y protección de recursos del CDN.

## [0.6.2] - 2026-09-02
- Acción para detener el mirror y popups de listas.

## [0.6.1] - 2026-09-02
- Versión usada durante pruebas de bundles JavaScript.
