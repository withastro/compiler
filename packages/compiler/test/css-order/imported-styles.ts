import { test } from 'uvu';
import * as assert from 'uvu/assert';
import { transform } from '@astrojs/compiler';

const FIXTURE = `
---
import '../styles/global.css';
---
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

test('Astro style imports placed after frontmatter imports', () => {
  let idx1 = result.code.indexOf('../styles/global.css');
  let idx2 = result.code.indexOf('test.astro?astro&type=style&index=0&lang.css');
  assert.ok(idx2 > idx1);
});

test.run();
