---
"@astrojs/compiler": minor
---

Adds an `walkAsync` utility function that returns a Promise from the tree traversal process. Unlike the existing `walk` function which doesn't provide a way to wait for traversal completion, `walkAsync` allows consumers to `await` the full traversal of the AST
