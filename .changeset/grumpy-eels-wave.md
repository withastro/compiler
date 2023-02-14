---
'@astrojs/compiler': patch
---

Fix incorrect `convertToTSX` types. The function accepts `filename`, not `sourcefile`.
