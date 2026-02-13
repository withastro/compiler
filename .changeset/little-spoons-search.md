---
"@astrojs/compiler": major
---

Removes the first argument of `$$result.createAstro()`

`$$result.createAstro()` does not accept an `AstroGlobalPartial` as the first argument anymore:

```diff
-const Astro = $$result.createAstro($$Astro, $$props, $$slots);
+const Astro = $$result.createAstro($$props, $$slots);
```
