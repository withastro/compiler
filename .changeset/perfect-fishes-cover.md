---
'@astrojs/compiler': patch
---

Fixes an issue where a `tr` element which contained an expression would cause its parent table to swallow any trailing element inside said table
