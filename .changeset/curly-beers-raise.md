---
'@astrojs/compiler': minor
---

The compiler will now return `errors` and `warnings` which can be handled by the consumer. For example:

```js
import { transform } from '@astrojs/compiler';

const { errors } = await transform(file, opts);
for (const error of errors) {
  console.error(error.text);
}
// or
const { code, warnings } = await transform(file, opts);
for (const warning of warnings) {
  console.warn(warning.text);
}
```
