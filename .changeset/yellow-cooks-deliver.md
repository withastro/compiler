---
'@astrojs/compiler': minor
---

Adds `serverComponents` metadata

This adds a change necessary to support server islands. During transformation the compiler discovers `server:defer` directives and appends them to the `serverComponents` array. This is exported along with the other metadata so that it can be used inside of Astro.
