# Changelog

Todos los cambios relevantes del proyecto se registran en este archivo a partir de la versión 0.6.6.

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

### Pendiente de la siguiente iteración
- Prioridad `seed > caché > origen` para recursos estáticos presentes en `/data/seed`.
- Dejar CSS y JS funcionales prácticamente byte a byte y mover el bloqueo de publicidad al nivel de origen/CSP.
- Fallback entre reproductor local y reproductor online cuando no exista copia local del episodio.
- Mejorar la detección de episodios locales con `<mediaId>_<episodio>_SUB.*` como coincidencia fuerte cuando el ID corresponda a AnimeAV1.

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