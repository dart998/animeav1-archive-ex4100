# Changelog

Todos los cambios relevantes del proyecto se registran en este archivo a partir de la versión 0.6.6.

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
- Commits históricos:
  - `3531b29f3ee0fd3e55426b5f01754dbf30fface4` — Deploy 0.6.2.
  - `950eb14314649085ff3f9413b3cdf6d9bb96c83c` — Set application version 0.6.2.
  - `8179f9cc3a8f5f1b606377bc69366a13f4254f53` — Style AV1 list popups and disabled controls.
  - `89d8536dca02fdf011dbf054fb5d7e0eac6dcd1f` — Add AV1 list popups and mirror stop button.
  - `85ac3f32bd1f5148d4a6569bf1a67f5e1fa0ea04` — Add mirror stop action.
  - `5c674c7a6b4ffd71a8f568a4b3ddcb19cbd163fd` — Test mirror without injected JavaScript guard.

## [0.6.1] - 2026-09-02
- Versión usada durante las pruebas de bundles JavaScript sin modificar para investigar los iconos ausentes.
- Commit: `8ade1d6f85708a7ca52af92ecba66add2a2f339c` — Deploy 0.6.1.

---

A partir de 0.6.6 cada subida de versión debe incluir su sección correspondiente en este archivo.