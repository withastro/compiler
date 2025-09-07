---
"@astrojs/compiler": minor
---

fixed a bug where the Astro compiler incorrectly handled the 'as' property name in Props interfaces.

This allows Astro components to use 'as' as a prop name (common pattern for polymorphic components) without breaking TypeScript type inference. The Props type is now correctly preserved when destructuring objects with an 'as'
property.