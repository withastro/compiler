---
"@astrojs/compiler": patch
---

Fix scoped CSS nesting so descendant selectors without `&` inside nested rules are not incorrectly re-scoped.
