import { test } from 'uvu';
import * as assert from 'uvu/assert';
import { transform } from '@astrojs/compiler';

const FIXTURE = `
<style>
  body { color: green; }
</style>
`;

let result;
test.before(async () => {
  result = await transform(FIXTURE, {
    sourcefile: 'test.astro',
  });
});

test('Astro style imports are included in the compiled JS', () => {
  let idx = result.code.indexOf('test.astro?astro&type=style&index=0&lang.css');
  assert.not.equal(idx, -1);
});

test.run();
