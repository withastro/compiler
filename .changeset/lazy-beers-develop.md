---
'@astrojs/compiler': patch
---

Fixes an issue where HTML and JSX comments lead to subsequent content being incorrectly treated as plain text when they have parent expressions.
