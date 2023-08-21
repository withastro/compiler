---
'@astrojs/compiler': major
---

The scope hash created by the compiler is now **lowercase**.

This aligns with the HTML spec of the attribute names, where they are lowercase by spec.

This change is needed because the compiler now creates data attributes that contain the hash in their name.
