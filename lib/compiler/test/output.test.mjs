/* eslint-disable no-console */
import { transform } from '@astrojs/compiler';

const sleep = (ms) => new Promise((res) => setTimeout(res, ms));

async function run() {
  let i = 0;
  const result = await transform(
    `<div xmlns:happy="https://example.com/schemas/happy">
    <img src="jolly.avif" happy:smile="sweet"/>
  </div>
  `,
    {
      site: undefined,
      sourcefile: '/Users/matthew/dev/astro/packages/astro/test/fixtures/astro-attrs/src/pages/namespaced.astro',
      sourcemap: 'both',
      internalURL: 'astro/internal',
      preprocessStyle: async (value, attrs) => {
        return null;
      },
    }
  );

  if(result.code[0] === '\x00') {
    throw new Error('Corrupt output');
  }
}

await run().catch(err => { console.error(err); process.exit(1); });
