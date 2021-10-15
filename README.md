# Astro Compiler

Astro’s [Go][go] + WASM compiler.

⚠️ Currently in beta!

## Install

```
npm install @astrojs/compiler
```

## Usage

_Note: API will change before 1.0! Use at your own discretion._

```js
import { transform } from "@astrojs/compiler";

const result = await transform(source, {
  site: "https://mysite.dev",
  sourcefile: "/Users/astro/Code/project/src/pages/index.astro",
  sourcemap: "both",
  internalURL: "astro/internal",
});
```

## Contributing

[CONTRIBUTING.md](./CONTRIBUTING.md)
