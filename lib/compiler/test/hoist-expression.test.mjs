/* eslint-disable no-console */
import { transform } from '@astrojs/compiler';

async function run() {
  const result = await transform(
    `---
const url = 'foo';
---
<script type="module" hoist src={url}></script>`,
    {
      sourcefile: 'src/pages/index.astro',
      sourcemap: true,
      experimentalStaticExtraction: true,
    }
  );

  console.assert(result);
}

await run();
