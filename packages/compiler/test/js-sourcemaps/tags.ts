import { test } from 'uvu';
import * as assert from 'uvu/assert';
import { testJsSourcemap } from '../utils';

test('tag close', async () => {
  const input = `<Hello></Hello>`;
  const output = await testJsSourcemap(input, '>');

  assert.equal(output, {
    line: 1,
    column: 6,
    source: 'index.astro',
    name: null,
  });
});

test.run();
