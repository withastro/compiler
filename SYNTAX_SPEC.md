# The `.astro` File Format — Syntax Specification

**Version:** 1.0
**Status:** Draft
**Date:** 2026-02-03

---

## Table of Contents

1. [File Structure](#1-file-structure)
2. [Component Script (Frontmatter)](#2-component-script-frontmatter)
3. [Template](#3-template)
4. [Style Blocks](#4-style-blocks)
5. [Script Blocks](#5-script-blocks)

---

## 1. File Structure

An `.astro` file is composed of up to two sections described below. All are optional. When present, they must appear in this order:

```
┌──────────────────────────────────┐
│  ---                             │
│  Component Script                │
│  ---                             │
├──────────────────────────────────┤
│  Template                        │
└──────────────────────────────────┘
```

### 1.1 Minimal examples

```astro
<!-- no script, no style — just HTML -->
<h1>Hello, World!</h1>
```

```astro
---
const greeting = "Hello";
---

<h1>{greeting}, World!</h1>

<style>
  h1 { color: royalblue; }
</style>
```

---

## 2. Component Script (Frontmatter)

The region between the two `---` fences.

- The opening and closing fences are not required on their own line. Code may appear on the same line as both fences.
- Only one component script block is allowed per file.
- Any amount of whitespace may appear before the opening fence or after the closing fence.
- Any content may appear before before the opening fence, but is customarily ignored.

The component script is TypeScript. All standard TypeScript syntax is valid, apart from the exceptions and additions outlined in §2.1.

### 2.1 Top-level return

`return` may be used at the top level:

```astro
---
import { getUser } from "../lib/auth.js";

const user = await getUser();
if (!user) {
  return Astro.redirect("/login");
}
---
```

---

## 3. Template

The template is considered to be everything after the closing fence of the component script, or the entire file when there is no component script.

The template mostly follows the [JSX specification](https://facebook.github.io/jsx/), with the differences and additions outlined in §3.1.

### 3.1 Differences from JSX

These differences apply both within the template and within expressions inside the template.

For instance, it is possible to use HTML comments inside an expression:

```astro
{ // JSX expression
  /* This is a JSX comment */
  <!-- This is an HTML comment -->
}
```

#### HTML comments

HTML comments `<!-- … -->` are allowed directly in the template (in addition to the standard JSX `{/* … */}` comments).

#### Multiple root elements

Unlike JSX, no single root element is required:

```astro
<header>…</header>
<main>…</main>
<footer>…</footer>

<!-- or inside an expression: -->
{
  <div>1</div>
  <div>2</div>
  <div>3</div>
}
```

#### Attribute names

Attribute names do not need to be valid JS identifiers. Characters like `@` and `.` are allowed:

```astro
<div @click="handler" x.data="value" />
```

#### Attribute shorthand

Attributes can use a shorthand syntax where `{prop}` expands to `prop={prop}`:

```astro
<Component {prop} />
<!-- equivalent to: -->
<Component prop={prop} />
```

#### Template literal attributes

Attributes can use backticks for interpolation without opening an expression:

```astro
<Component attribute=`hello ${value}` />
```

#### Unclosed HTML tags

HTML void elements do not need to be self-closed:

```astro
<input type="text">
<br>
<img src="image.png">
```

#### All HTML tags supported

Astro supports all HTML tags, including `<script>` and `<style>`. See §4 and §5 for details on their syntax.

---

## 4. Style Blocks

```astro
<style>
  h1 { color: red; }
</style>
```

Multiple `<style>` blocks are allowed per file.

### 4.1 Language

By default, `<style>` blocks can contain CSS. The content adheres to standard CSS syntax as defined by the [CSS Syntax Module](https://www.w3.org/TR/css-syntax-3/).

### 4.2 `lang` attribute

Specifies a preprocessor language:

```astro
<style lang="scss">
  $accent: #1d4ed8;
  .card { border-color: $accent; }
</style>
```

The syntax then follows the rules of the specified preprocessor instead of standard CSS.

---

## 5. Script Blocks

```astro
<script>
  console.log("Hello");
</script>
```

Multiple `<script>` blocks are allowed per file.

### 5.1 Language

A bare `<script>` tag with no attributes, can contains TypeScript. The content adheres to standard TypeScript syntax.

```astro
<script>
  interface User {
    id: number;
    name: string;
  }

  // ...
</script>
```

If any attributes are present, the content follows standard [HTML `<script>` element](https://html.spec.whatwg.org/multipage/scripting.html#the-script-element) rules.

```astro
<script defer>
	// JavaScript
</script>

<script type="module">
  // JavaScript module
  import { foo } from "./foo.js";
</script>

<script type="application/json">
  { "key": "value" }
</script>
```
