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
  assert.ok(Array.isArray(result.errors));
  assert.is(result.errors.length, 0);
  assert.is(result.warnings.length, 1);
  assert.match(result.warnings[0].text, 'Unclosed tag');
  assert.is(result.warnings[0].location.lineText, '<div');
});

test.run();
