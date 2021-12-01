/* eslint-disable no-console */
import { transform } from '@astrojs/compiler';

const contents = `
{Astro.slots.a && <div id="a">
  <slot name="a" />
</div>}
`;

async function run() {
  const result = await transform(contents, {
    sourcemap: true,
    as: 'fragment',
    site: undefined,
    sourcefile: 'MoreMenu.astro',
    sourcemap: 'both',
    internalURL: 'astro/internal',
  });

  console.log(result.code);
}

await run();
