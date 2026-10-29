---
"@astrojs/compiler": patch
---

Fixes an issue where comments containing `<` or `>` in frontmatter could cause `Props` to degrade to `Record<string, any>` in TSX output
