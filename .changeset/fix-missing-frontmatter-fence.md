---
"@astrojs/compiler": patch
---

Fixes a panic when parsing files with a closing frontmatter fence (---) but no opening fence. The compiler now returns a helpful diagnostic error instead of crashing.
