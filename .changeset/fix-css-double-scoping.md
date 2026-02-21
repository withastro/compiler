---
"@astrojs/compiler": patch
---

Fixes a CSS scoping regression where selectors using the nesting selector (`&`) with pseudo-classes or pseudo-elements (e.g. `&:last-of-type`, `&::before`) inside `:global()` contexts would incorrectly receive a duplicate scope attribute.
