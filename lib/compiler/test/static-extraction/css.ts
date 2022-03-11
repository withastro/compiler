import { test } from 'uvu';
import * as assert from 'uvu/assert';
import { transform } from '@astrojs/compiler';

const FIXTURE = `
---
---
<style>
    .thing { color: green; }
</style>
`;

let result;
test.before(async () => {
  result = await transform(FIXTURE, {
    experimentalStaticExtraction: true,
  });
});

test('extracts styles', () => {
  assert.equal(result.css.length, 1, `Incorrect CSS returned. Expected a length of 1 and got ${result.css.length}`);
});

test.run();
