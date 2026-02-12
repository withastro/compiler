# Contributing

Contributions are welcome to the Astro compiler!

## Setup

### Rust

[Rust][rust] is needed to work with this repo. Install it via [rustup][rustup].

### Node

You will also need [Node.js][node] installed, as well as pnpm (`npm i -g pnpm`). Run `pnpm install` to install dependencies.

## Code Structure

The compiler is split into two Rust crates:

- `crates/astro_codegen/` - The core code generation engine that transforms Astro ASTs into JavaScript
- `crates/astro_napi/` - Node.js NAPI bindings that expose the compiler to JavaScript

The compilation pipeline is:

1. **Parsing** - `oxc_parser` (from the oxc project) parses `.astro` files into an AST
2. **Scanning** - `AstroScanner` pre-analyzes the AST to collect metadata (hydrated components, scripts, etc.)
3. **Printing** - `AstroCodegen` generates JavaScript code from the AST

The `packages/compiler/` TypeScript package provides the `@astrojs/compiler` npm API, wrapping the NAPI bindings.

## Building

```shell
# Build the NAPI native addon (debug mode)
pnpm run build:napi

# Build the TypeScript package
pnpm run build:compiler

# Build everything
pnpm run build:all
```

## Tests

### Rust tests

```shell
# Run all Rust tests (unit + snapshot)
cargo test

# Run only astro_codegen tests
cargo test -p astro_codegen

# Update snapshots after changes
cargo insta review
```

### TypeScript tests

```shell
pnpm run test
```

### Adding new test cases

Add a new `.astro` fixture file in `crates/astro_codegen/tests/fixtures/` and run:

```shell
cargo test -p astro_codegen
```

A new `.snap` file will be created. Review it with `cargo insta review`.

[rust]: https://www.rust-lang.org/
[rustup]: https://rustup.rs/
[node]: https://nodejs.org/
