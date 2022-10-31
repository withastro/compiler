import { test } from 'uvu';
import * as assert from 'uvu/assert';
import { testJsSourcemap } from '../utils';

test('shorthand attribute', async () => {
  const input = `<div {name} />`;

  const output = await testJsSourcemap(input, 'name');
  assert.equal(output, {
    source: 'index.astro',
    line: 1,
    column: 6,
    name: null,
  });
});

test('empty quoted attribute', async () => {
  const input = `<div src="" />`;

  const open = await testJsSourcemap(input, '"');
  assert.equal(open, {
    source: 'index.astro',
    line: 1,
    column: 9,
    name: null,
  });
});

test.run();
