---
'@astrojs/compiler': minor
---

Update `script` and `style` behavior to adhere to [RFC0008](https://github.com/withastro/rfcs/blob/main/proposals/0008-style-script-behavior.md#new-rfc-behavior).

In practice, this means that `script` and `style` elements that are not at the top-level of a template are treated as `is:inline`.
