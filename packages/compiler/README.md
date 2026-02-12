# Astro Compiler

Astro's compiler, written in Rust with [NAPI-RS](https://napi.rs/) bindings for Node.js.

## Install

```
npm install @astrojs/compiler
```

## Usage

#### Transform `.astro` to JavaScript

The Astro compiler transforms `.astro` component files into JavaScript modules whose default export generates HTML.

```js
import { transform } from "@astrojs/compiler";

const result = transform(source, {
  filename: "/Users/astro/Code/project/src/pages/index.astro",
  sourcemap: "both",
});
```

#### Parse `.astro` and return an AST

The compiler can emit an ESTree-compatible AST using the `parse` method.

```js
import { parse } from "@astrojs/compiler";

const result = parse(source);

console.log(JSON.stringify(result.ast, null, 2));
```

## Contributing

**New contributors welcome!** Check out our [Contributors Guide](CONTRIBUTING.md) for help getting started.

Join us on [Discord](https://astro.build/chat) to meet other maintainers. We'll help you get your first contribution in no time!

## Links

- [License (MIT)](LICENSE)
- [Code of Conduct](https://github.com/withastro/.github/blob/main/CODE_OF_CONDUCT.md)
- [Open Governance & Voting](https://github.com/withastro/.github/blob/main/GOVERNANCE.md)
- [Project Funding](https://github.com/withastro/.github/blob/main/FUNDING.md)
- [Website](https://astro.build/)

## Sponsors

Astro is free, open source software made possible by these wonderful sponsors.

[❤️ Sponsor Astro! ❤️](https://github.com/withastro/.github/blob/main/FUNDING.md)

<p align="center">
  <a target="_blank" href="https://opencollective.com/astrodotbuild">
    <img src="https://astro.build/sponsors.png" alt="Sponsor logos including the current Astro Sponsors, Gold Sponsors, and Exclusive Partner Sponsors: Netlify, Sentry, and Project IDX." />
  </a>
</p>