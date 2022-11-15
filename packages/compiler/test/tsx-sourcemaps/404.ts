import { test } from 'uvu';
import * as assert from 'uvu/assert';
import { convertToTSX } from '@astrojs/compiler';

test('404 generates a valid identifier', async () => {
  const input = `<div {name} />`;

  const output = await convertToTSX(input, { sourcefile: '404.astro', sourcemap: 'inline' });
  assert.match(output.code, `export default function __AstroComponent_`);
});
