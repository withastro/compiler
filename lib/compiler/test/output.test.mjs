/* eslint-disable @typescript-eslint/no-unused-vars */
/* eslint-disable no-console */
import { transform } from '@astrojs/compiler';

async function run() {
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
      preprocessStyle: async (_value, _attrs) => {
        return null;
      },
    }
  );

  if (result.code[0] === '\x00') {
    throw new Error('Corrupt output');
  }
}

await run().catch((err) => {
  console.error(err);
  process.exit(1);
});
