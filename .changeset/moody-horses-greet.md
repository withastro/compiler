---
'@astrojs/compiler': patch
---

Reset tokenizer state when a raw element that is self-closing is encountered. 

This fixes the handling of self-closing elements like `<title />` and `<script />` when used with `set:html`.
