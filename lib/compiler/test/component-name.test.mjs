/* eslint-disable no-console */

import { transform } from '@astrojs/compiler';

async function run() {
  const result = await transform(`<div>Hello world!</div>`, {
    sourcemap: true,
    pathname: '/src/components/Cool.astro',
  });

  if (!result.code.includes('export default $$Cool')) {
    throw new Error(`Expected component export to be named "Cool"!`);
  }
}

await run();
