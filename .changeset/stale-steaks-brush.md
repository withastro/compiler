---
'@astrojs/compiler': minor
---

Add a new `annotateSourceFile` option. This options makes it so the compiler will annotate every element with their source file location. This is notably useful for dev tools to be able to provide features like a "Open in editor" button. This option is disabled by default.

```ts
<div>
  <span>hello world</span>
</div>
```

Results in:

```ts
<div data-astro-source-file="/Users/erika/Projects/..." data-astro-source-loc="1:1">
  <span data-astro-source-file="/Users/erika/Projects/..." data-astro-source-loc="2:2">
    hello world
  </span>
</div>
```

In Astro, this option is enabled only in development mode.
