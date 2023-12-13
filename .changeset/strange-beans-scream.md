---
'@astrojs/compiler': patch
---

Fixes an issue causing `index out of range` errors when handling some multibyte characters like `\u2028`.