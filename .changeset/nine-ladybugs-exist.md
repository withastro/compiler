---
'@astrojs/compiler': patch
---

Replace `astro.ParseFragment` in favor of `astro.ParseFragmentWithOptions`.

We now check whether an error handler is passed when calling `astro.ParseFragmentWithOptions`
