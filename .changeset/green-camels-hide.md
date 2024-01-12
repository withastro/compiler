---
'@astrojs/compiler': minor
---

- Adds support for dynamic slots inside loops
- Fixes an issue where successive named slotted elements would cause a runtime error
- Fixes an issue in which if there was an implicit default slotted element next to named one, the former would get swallowed by the later.
