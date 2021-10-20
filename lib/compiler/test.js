import { createRequire } from 'module';
import { pathToFileURL } from 'url';

const require = createRequire(import.meta.url);

(async () => {
  const { transform } = await import('@astrojs/compiler');
  const template = await transform(
    `---
let value = 'world';
---
<h1>Hello {value.split('').reverse().join('')}!</h1>`,
    { sourcemap: true }
  );
  // const start = performance.now()
  // const html = await compile(template);
  // const end = performance.now()
  console.log(template);

  // console.log('Compiled in ' + (start - end).toFixed(1) + 'ms');
  // console.log(html);
})();
