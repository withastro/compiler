---
"@astrojs/compiler": patch
---

Fixes a bug where trailing whitespaces were preserved before `<style>` tags after transformation, in certain cases. Now trailing whitespaces are correctly removed.
