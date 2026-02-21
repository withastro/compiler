---
"@astrojs/compiler": patch
---

Fixes an issue where `server:defer` was treated like a transition directive, causing ViewTransitions CSS to be included even when no `transition:*` directives were used.
