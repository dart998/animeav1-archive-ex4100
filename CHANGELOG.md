# Changelog

Todos los cambios relevantes del proyecto se registran en este archivo a partir de la versión 0.6.6.

## [0.6.9] - 2026-09-10

### Añadido
- Matching local por franquicia y alias significativo para casos como DanMachi.
- Resolución de temporada a partir de títulos/aliases y subcarpetas como `Temporada 2`, `Season 2` o `S02`, evitando mezclar episodios entre temporadas.
- Contador real de recursos existentes bajo `/data/site` y contador separado de archivos guardados durante la sincronización actual.
- Popup de recursos guardados con árbol desplegable organizado por carpetas, nombre y tamaño de archivo.
- Vista de logs del mirror con aspecto de terminal, incluyendo recursos guardados y errores recientes.
- Endpoint `/api/mirror/resources` para consultar inventario y log de recursos del mirror.

### Cambiado
- El panel de administración usa el matcher multi-carpeta/franquicia en lugar de depender exclusivamente de coincidencia exacta.
- Se mejoran las cabeceras usadas al solicitar imágenes a `cdn.animeav1.com`, con User-Agent de navegador, `Accept` de imagen y cabeceras `Sec-Fetch-*`, sin reenviar la cookie de AnimeAV1.
- El README se reescribe para reflejar el funcionamiento actual, sin referencias a la prueba inicial ni números de versión fijos.
- El contador de elementos sanitizados se sustituye por **Guardados en esta sincro**.
- Se eliminan los tests unitarios y el paso `go test ./...` del workflow para reducir el tiempo de generación de versiones. La imagen ARMv7 sigue compilándose en GitHub Actions.

### Pendiente
- Marcar y desmarcar favoritos desde las fichas usando la acción real de AnimeAV1.
- Fallback entre reproductor local y reproductores online legítimos.

## [0.6.8] - 2026-09-09

### Añadido
- Prioridad efectiva `seed > caché > origen` para recursos estáticos conocidos. Al arrancar, los CSS/JS/SVG/imágenes/fuentes existentes en `/data/seed` se promocionan sobre `/data/site`; HTML y JSON nunca se sobrescriben desde el seed.
- Detección local multi-carpeta por serie para agrupar temporadas, Parts, OVAs/OADs/especiales relacionados sin mover ni renombrar archivos.
- Coincidencia fuerte de episodios descargados de AnimeAV1 mediante `<mediaId>_<episodio>_...`, por delante de patrones genéricos como `S01E05`, `Ep05` o números aislados.
- Tests de regresión para prioridad del seed, reescritura CDN, namespaces SVG, parche post-hidratación, prioridad de listas y matching multi-carpeta/mediaId.
- El workflow ejecuta `go test ./...` antes de QEMU/build/push; si falla un test, no se publica imagen Docker.

### Cambiado
- La reproducción local busca recursivamente en todas las carpetas relacionadas con la serie y prioriza primero coincidencias por `mediaId`, después patrones explícitos de episodio y finalmente números aislados.
- Una carpeta que coincide exactamente con el título/alias de AnimeAV1 tiene prioridad sobre carpetas relacionadas cuando el patrón de archivo tiene la misma calidad.

### Seguridad y compatibilidad
- El seed solo promociona extensiones estáticas conocidas y deja las fichas HTML dinámicas bajo control de AnimeAV1/mirror.
- Se mantiene la reescritura post-hidratación del CDN a `/_cdn/...`, evitando que las imágenes restauradas por SvelteKit salten la caché local.

## [0.6.7] - 2026-09-09

### Corregido
- Las imágenes del CDN que SvelteKit vuelve a convertir en URLs absolutas después de hidratar la página se reescriben otra vez a `/_cdn/...`, manteniendo la prioridad de la copia local y descargando desde CDN solo cuando falta el recurso.
- El botón de actualización individual de la ficha se vuelve a insertar si la hidratación de SvelteKit reemplaza el bloque donde estaba situado.

### Cambiado
- La descarga/actualización del sitio prioriza las series según las listas de AnimeAV1 en este orden: **Viendo → Planeado → Completado → resto**.
- La prioridad se aplica al inicio de la cola del mirror sin excluir el resto del contenido descubrible.

## [0.6.6] - 2026-09-09

### Añadido
- Botón de actualización individual en las fichas `/media/<slug>`, insertado junto al control de compartir y reutilizando su estilo visual.
- Endpoint interno `POST /__mirror/refresh?path=/media/<slug>` para refrescar únicamente la ficha seleccionada sin lanzar una sincronización completa del mirror.
- Registro de cambios histórico del proyecto en `CHANGELOG.md`.

### Pendiente
- Fallback entre reproductor local y reproductor online cuando no exista copia local del episodio.

## [0.6.5] - 2026-09-09
- Reproducción de episodios locales desde la biblioteca montada en `/library`.
- Sincronización de episodios vistos con AnimeAV1 mediante la acción SvelteKit real de `/cuenta/listas?/library`.
- La cookie de AnimeAV1 permanece exclusivamente en backend.
- Commit: `58b117227d86a1cf2af01b6e1f90a2a9c6672698` — Add local episode playback and AnimeAV1 watched sync.

## [0.6.4] - 2026-09-08
- Mirror incremental y reanudable: detener una sincronización ya no elimina el progreso descargado.
- Reutilización de recursos estáticos y páginas de episodios existentes.
- `onclick`, `onmousedown` y `onmouseup` dejan de considerarse publicidad por sí solos.
- Commit: `ddb79dd434d6db396508b5a8ff9d5725eef91311` — Make mirror incremental and preserve UI mouse events.

## [0.6.3] - 2026-09-06
- Fallback local desde `/data/seed` para recursos que AnimeAV1/CDN rechazan con 403/404.
- Referer específico para `cdn.animeav1.com` sin reenviar la cookie de sesión.
- Protección de namespaces SVG legítimos frente a la neutralización genérica de URLs.
- Commit: `eb07d578623a3cd6b288efd85a67705effe94105` — Add seed fallback and release 0.6.3.

### Despliegue posterior a 0.6.3
- `520a59c043a41c362a1bad3c43fb40998f862009` — Remove Portainer webhook deployment.
- `7842adbc4503204f5ee846de846fc951d3dfd76f` — Document Portainer polling deployment.

## [0.6.2] - 2026-09-02
- Acción para detener la descarga del mirror.
- Popups de las listas AnimeAV1 y controles deshabilitados estilizados.
- Experimento sin `localGuard` inyectado para aislar el problema de iconos.

## [0.6.1] - 2026-09-02
- Versión usada durante las pruebas de bundles JavaScript sin modificar para investigar los iconos ausentes.

---

A partir de 0.6.6 cada subida de versión debe incluir su sección correspondiente en este archivo.
