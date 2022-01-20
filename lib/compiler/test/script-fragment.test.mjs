/* eslint-disable no-console */

import { transform } from '@astrojs/compiler';

async function run() {
  const result = await transform(`<script src={Astro.resolve("../scripts/no_hoist_nonmodule.js")}></script>`, {
    sourcemap: true,
    site: undefined,
    sourcefile: 'MoreMenu.astro',
    sourcemap: 'both',
    internalURL: 'astro/internal',
  });

  if (!result.code) {
    throw new Error('Unable to compile script fragment!');
  }
}

await run();
