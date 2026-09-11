# Changelog

Todos los cambios relevantes del proyecto se registran en este archivo a partir de la versión 0.6.6.

## [0.6.14] - 2026-09-12

### Añadido
- Notificaciones de episodios nuevos para series en estado **Viendo**: el crawler crea una notificación cuando descubre episodios posteriores al último conjunto conocido, cada aviso enlaza directamente al episodio y la campana nativa muestra contador, iluminación y animación de tintineo mientras haya avisos sin leer.
- El botón nativo **Leer todo** marca todas las notificaciones como leídas. Las leídas se conservan 24 horas y después se eliminan automáticamente; las no leídas no caducan.
- Si existen episodios locales en un formato que el navegador no puede reproducir, al iniciar una descarga se ofrece volver a descargarlos. La copia existente nunca se borra ni se sobrescribe.

### Corregido
- Cerrar el modal de descarga ya no hace que vuelva a abrirse por el polling de progreso.
- El botón de descarga deja de girar: durante una descarga la flecha baja repetidamente hacia la línea mientras la base permanece fija.
- Los botones añadidos por el mirror dejan de usar `title`, evitando el segundo tooltip negro retardado del navegador.
- Después de completar cada episodio descargado se reindexa inmediatamente `/library`, de forma que pasa a estar disponible como copia local sin esperar a que termine toda la serie.
- Si el nombre de destino de una nueva descarga ya existe se crea un nombre alternativo, sin sobrescribir archivos existentes.

### Interfaz
- Se elimina el enlace a Discord del encabezado.
- Se eliminan del pie los enlaces **Términos y condiciones** y **Política de privacidad**.

## [0.6.13] - 2026-09-11

### Corregido
- La reproducción remota deja de mostrar una segunda barra propia bajo el reproductor. Se reutilizan los botones originales de AnimeAV1 y solo puede quedar un origen activo a la vez.
- HLS/Zilla y UPNShare/uns.bio desaparecen del selector del mirror. Los demás proveedores de streaming reales se conservan según estén disponibles en cada episodio.
- Las fuentes de streaming y los enlaces de descarga quedan separados: Transfer.it y 1fichier nunca se usan como reproductores, y Mega solo se considera streaming cuando la URL es de tipo `/embed/`.
- El matcher local reconoce mejor títulos de temporadas con sufijos numéricos y prioriza carpetas existentes con contenido antes de crear una nueva.
- Antes de decidir el destino de una descarga se vuelve a escanear `/library`, evitando depender de un índice desactualizado.
- Las carpetas nuevas y los archivos descargados se crean con permisos de escritura compatibles con la gestión posterior desde SMB.

### Añadido
- Botón **Local** integrado en la barra original de proveedores cuando existe una copia local del episodio. Si el formato es reproducible por el navegador, Local es el origen inicial y queda iluminado; se puede alternar libremente entre Local y los proveedores remotos y volver a Local en cualquier momento.
- Si existe una copia local no reproducible directamente por el navegador, como MKV/AVI, el botón Local aparece deshabilitado con explicación y se usa un proveedor remoto compatible.
- El botón **Descargar todos** abre un modal integrado con el estilo de la página mostrando estado por episodio, progreso en bytes y porcentaje, completados, ya existentes y errores.
- El estado de la descarga continúa en backend aunque se cierre el modal o se abandone la página, y se persiste en SQLite para poder consultar posteriormente el resultado o detectar una interrupción por reinicio del contenedor.
- La descarga masiva resuelve el enlace de descarga Mega episodio por episodio, independiente de los reproductores de streaming.

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
