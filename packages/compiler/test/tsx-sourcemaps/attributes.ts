import { test } from 'uvu';
import * as assert from 'uvu/assert';
import { testSourcemap } from '../utils';

test('shorthand attribute', async () => {
  const input = `<div {name} />`;

  const output = await testSourcemap(input, 'name');
  assert.equal(output, {
    source: 'index.astro',
    line: 1,
    column: 6,
    name: null,
  });
});

test.run();
