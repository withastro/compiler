---
'@astrojs/compiler': minor
---

Improve error recovery when using the `transform` function. The compiler will now properly reject the promise with a useful message and stacktrace rather than print internal errors to stdout.
