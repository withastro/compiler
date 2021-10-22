# @astrojs/compiler

## 0.2.12

### Patch Changes

- c9fa9eb: Fix for apostrophe within elements

## 0.2.11

### Patch Changes

- 27629b2: Reverts the apostrophe change that broke markdown parsing

## 0.2.10

### Patch Changes

- 57eb728: Fixes hydrated scripts not recognized when using fragment transformation

## 0.2.9

### Patch Changes

- 3ea8d8c: Fix for string interpolation within titles
- ef7cb1e: Fixes bug with textContent containing apostrophe character

## 0.2.8

### Patch Changes

- b2d5564: Fixes wasm build

## 0.2.6

### Patch Changes

- fix small issue with `preprocessStyle` handling of `null` or `undefined`

## 0.2.5

### Patch Changes

- Fix issue with CI deployment

## 0.2.4

### Patch Changes

- 4410c5a: Add support for a `preprocessStyle` function
- 934e6a6: Chore: add linting, format code

## 0.1.15

### Patch Changes

- 5c02abf: Fix split so it always splits on first non-import/export
- 93c1cd9: Bugfix: handle RegExp in Astro files
- 94c59fa: Bugfix: tokenizer tries to parse JS comments
- 46a5c75: Adds the top-level Astro object
- 7ab9148: Improve JS scanning algorithm to be more fault tolerant, less error prone

## 0.1.12

### Patch Changes

- 96dc356: Adds hydrationMap support for custom elements

## 0.1.11

### Patch Changes

- 939283d: Adds the component export for use in hydration

## 0.1.10

### Patch Changes

- 3a336ef: Adds a hydration map to enable hydration within Astro components

## 0.1.9

### Patch Changes

- 7d887de: Allows the Astro runtime to create the Astro.slots object

## 0.1.8

### Patch Changes

- d159658: Publish via PR

## 0.1.7

### Patch Changes

- c52e69b: Include astro.wasm in the package

## 0.1.6

### Patch Changes

- bd05f7c: Actually include _any_ files?

## 0.1.5

### Patch Changes

- c4ed69e: Includes the wasm binary in the npm package

## 0.1.4

### Patch Changes

- 2f1f1b8: Pass custom element tag names to renderComponent as strings

## 0.1.3

### Patch Changes

- e4e2de5: Update to [`tinygo@0.20.0`](https://github.com/tinygo-org/tinygo/releases/tag/v0.20.0) and remove `go@1.16.x` restriction.
- ae71546: Add support for `fragment` compilation, to be used with components rather than pages
- 8c2aaf9: Allow multiple top-level conditional expressions

## 0.1.0

### Patch Changes

- c9407cd: Fix for using conditionals at the top-level
