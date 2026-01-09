---
"@astrojs/compiler": patch
---

Use JSX/TSX style attribute deduplication

Changed compiler to use the last occurrence of duplicate props instead of first, matching standard JSX semantics where later values override earlier ones. This ensures correct prop precedence for patterns like `<div {...props} className="override" />`.

Before: <div a="1" a="2" /> → <div a="1" />
After: <div a="1" a="2" /> → <div a="2" />
