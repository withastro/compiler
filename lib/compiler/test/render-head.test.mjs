/* eslint-disable @typescript-eslint/no-unused-vars */
/* eslint-disable no-console */
import { transform } from '@astrojs/compiler';

const source = `<html><head><title>Ah</title></head></html>`;

async function run() {
  const result = await transform(source, {
    site: undefined,
    sourcefile: '/Users/matthew/dev/astro/packages/astro/test/fixtures/astro-attrs/src/pages/namespaced.astro',
    sourcemap: 'both',
    internalURL: 'astro/internal',
    preprocessStyle: async (_value, _attrs) => {
      return null;
    },
  });

  console.log(result.code);
  if (!result.code.includes(`<!--astro:head-->`)) {
    throw new Error('Result did not include <!--astro:head--> placeholder');
  }
}

await run().catch((err) => {
  console.error(err);
  process.exit(1);
});
