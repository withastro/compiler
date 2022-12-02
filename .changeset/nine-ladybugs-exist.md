---
'@astrojs/compiler': patch
---

Internally, replace `astro.ParseFragment` in favor of `astro.ParseFragmentWithOptions`. We now check whether an error handler is passed when calling `astro.ParseFragmentWithOptions`
