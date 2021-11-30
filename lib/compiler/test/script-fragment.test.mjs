/* eslint-disable no-console */

import { transform } from '@astrojs/compiler';

async function run() {
  const result = await transform(
    `<script src={Astro.resolve("../scripts/no_hoist_nonmodule.js")}></script>`,
    {
      sourcemap: true,
      as: 'fragment',
      site: undefined,
      sourcefile: 'MoreMenu.astro',
      sourcemap: 'both',
      internalURL: 'astro/internal',
    }
  );

  console.log(result.code)
}

await run();
