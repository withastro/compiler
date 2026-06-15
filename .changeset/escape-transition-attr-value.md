---
"@astrojs/compiler": patch
---

Escape quoted `transition:name` and `transition:animate` attribute values when emitting the `renderTransition` call, so a value containing a double quote no longer breaks out of the generated JS string literal.
