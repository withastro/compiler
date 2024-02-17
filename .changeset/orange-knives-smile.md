---
'@astrojs/compiler': patch
---

Adds warnings indicating that the `data-astro-rerun` attribute can not be used on external ECMAScript Modules and that `data-astro-reload` is only supported on `<a>`, `<area>` and `<form>` elements.