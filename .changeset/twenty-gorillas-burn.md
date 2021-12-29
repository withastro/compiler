---
'@astrojs/compiler': patch
---

Fix a logical error with expression tokenization when using nested functions. Previously, only the first brace pair would be respected and following pairs would be treated as expression boundaries.
