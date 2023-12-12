---
'@astrojs/compiler': patch
---

Fixes an issue where when there are nested expressions, subsequent content was incorrectly treated as plain text in some cases.