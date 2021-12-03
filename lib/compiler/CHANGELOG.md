# @astrojs/compiler

## 0.5.7

### Patch Changes

- 75bd730: Fix regression with Components mixed with active formatting elements

## 0.5.6

### Patch Changes

- 7ca419e: Improve behavior of empty expressions in body and attributes, where `{}` is equivalent to `{(void 0)}`

## 0.5.5

### Patch Changes

- 7a41d7b: Fix `<>` syntax edge case inside of expressions
- b0d35b9: Fix edge case with conditional scripts

## 0.5.4

### Patch Changes

- f2e0322: Do not reconstruct active formatting elements on expression start
- 0103285: Bugfix: expressions in table context

## 0.5.3

### Patch Changes

- 50cbc57: Fix fragment expression behavior edge case

## 0.5.2

### Patch Changes

- 8f0e3d7: Fix fragment parsing bugs when frontmatter is missing or top-level expressions are present

## 0.5.1

### Patch Changes

- 1f0ba41: Fix bug when fragment parsing frontmatter is missing

## 0.5.0

### Minor Changes

- 901faef: Passes projectRoot to createAstro

## 0.4.0

### Minor Changes

- 7e1aded: Change behavior of `as: "fragment"` option to support arbitrary `head` and `body` tags

## 0.3.9

### Patch Changes

- 2884a82: Bugfix: CSS comments insert semicolon

## 0.3.8

### Patch Changes

- 2c8f5d8: Fix another component-only edge case

## 0.3.7

### Patch Changes

- eb0d17f: Fix edge case with files that contain a single component

## 0.3.6

### Patch Changes

- af003e9: Fix syntax error in transformed output

## 0.3.5

### Patch Changes

- bca7e00: Fixed issue where an Astro Components could only add one style or script
- 2a2f951: Fix regression where leading `<style>` elements could break generated tags
- db162f8: Fix case-sensitivity of void elements
- 44ee189: Fixed issue where expressions did not work within SVG elements
- 9557113: Fix panic when preprocessed style is empty

## 0.3.4

### Patch Changes

- 351f298: Fix edge case with with `textarea` inside of a Component when the document generated an implicit `head` tag
- 0bcfd4b: Fix CSS scoping of \* character inside of calc() expressions
- 4be512f: Encode double quotes inside of quoted attributes
- ad865e5: Fix behavior of expressions inside of <table> elements

## 0.3.3

### Patch Changes

- 6d2a3c2: Fix handling of top-level component nodes with leading styles
- 2ce10c6: Fix "call to released function" issue

## 0.3.2

### Patch Changes

- 8800f80: Fix comments and strings inside of attribute expressions

## 0.3.1

### Patch Changes

- 432eaaf: Fix for compiler regression causing nil pointer

## 0.3.0

### Minor Changes

- 1255477: Drop support for elements inside of Frontmatter, which was undefined behavior that caused lots of TypeScript interop problems

### Patch Changes

- 44dc0c6: Fixes issue with \x00 character on OSX
- d74acfa: Fix regression with expressions inside of <select> elements
- f50ae69: Bugfix: don’t treat import.meta as import statement

## 0.2.27

### Patch Changes

- 460c1e2: Use `$metadata.resolvePath` utility to support the `client:only` directive

## 0.2.26

### Patch Changes

- 3e5ef91: Implement getStaticPaths hoisting
- 8a434f9: Fix namespace handling to support attributes like `xmlns:xlink`

## 0.2.25

### Patch Changes

- 59f36cb: Fix custom-element slot behavior to remain spec compliant
- 79b2e6f: Fix style/script ordering
- 6041ee5: Add support for `client:only` directive
- 2cd35f6: Fix apostrophe handling inside of elements which are inside of expressions ([#1478](https://github.com/snowpackjs/astro/issues/1478))

## 0.2.24

### Patch Changes

- bfd1b94: Fix issue with `style` and `script` processing where siblings would be skipped
- 726d272: Fix <Fragment> and <> handling
- f052465: Fix CSS variable parsing in the scoped CSS transform

## 0.2.23

### Patch Changes

- 632c29b: Fix nil pointer dereference when every element on page is a component
- 105a159: Fix bug where text inside of elements inside of an expression was not read properly (https://github.com/snowpackjs/astro/issues/1617)

## 0.2.22

### Patch Changes

- 04c1b63: Fix bug with dynamic classes

## 0.2.21

### Patch Changes

- 7b46e9f: Revert automatic DOCTYPE injection to fix package

## 0.2.20

### Patch Changes

- 39298e4: Fix small bugs with script/style hoisting behavior
- bd1014a: Bugfix: style tags in SVG

## 0.2.19

### Patch Changes

- 318dd69: Fix handling of self-closing "raw" tags like <script /> and <style />
- 9372c10: Support `define:vars` with root `html` element on pages
- c4491cd: Fix bug with <script define:vars> when not using the `hoist` attribute

## 0.2.18

### Patch Changes

- 2f4b772: Prevents overrunning an array when checking for raw attribute

## 0.2.17

### Patch Changes

- 4f9155a: Bugfix: fix character limit of 4096 characters
- 83df04c: Upgrade to Go 1.17

## 0.2.16

### Patch Changes

- 9ad8da7: Allows a data-astro-raw attr to parse children as raw text
- 61b77de: Bugfix: CSS and selector scoping

## 0.2.15

### Patch Changes

- 8fbae5e: Bugfix: fix component detection bug in parser
- 37b5b6e: Bugfix: wait to release processStyle() until after fn call

## 0.2.14

### Patch Changes

- f59c886: Bugfix: allow for detection of void tags (e.g. <img>)
- 4c8d14a: Fixes textContent containing a forward slash

## 0.2.13

### Patch Changes

- f262b61: Fix for string template usage within expressions

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
