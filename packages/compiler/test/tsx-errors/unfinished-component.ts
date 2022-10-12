import { test } from 'uvu';
import * as assert from 'uvu/assert';
import { convertToTSX } from '@astrojs/compiler';

const FIXTURE = `<div class={`;

let result;
test.before(async () => {
  result = await convertToTSX(FIXTURE, {
    pathname: '/src/components/unfinished.astro',
  });
});

test('did not crash on unfinished component', () => {
  assert.ok(result);
  assert.ok(Array.isArray(result.diagnostics));
  assert.is(result.diagnostics.length, 0);
});

test.run();
