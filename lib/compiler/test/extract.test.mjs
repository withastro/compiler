/* eslint-disable no-console */

import { transform } from '@astrojs/compiler';

async function run() {
  const result = await transform(
    `---
---
<style>
    .thing { color: green; }
</style>`,
    {
      sourcemap: true,
      site: undefined,
      sourcefile: 'MoreMenu.astro',
      sourcemap: 'both',
      internalURL: 'astro/internal',
      experimentalStaticExtraction: true,
    }
  );

  const cssLen = result.css.length;
  if (cssLen !== 1) {
    throw new Error(`Incorrect CSS returned. Expected a length of 1 and got ${cssLen}`);
  }
}

await run();
