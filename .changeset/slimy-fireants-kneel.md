---
'@astrojs/compiler': patch
---

Fix `parse` causing a compiler panic when a component with a client directive was imported but didn't have a matching import
