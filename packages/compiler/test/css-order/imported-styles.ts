import { transform } from '@astrojs/compiler';
import { test } from 'uvu';
import * as assert from 'uvu/assert';

const FIXTURE = `
---
import '../styles/global.css';
---
<style>
  body { color: green; }
</style>
`;

let result: unknown;
test.before(async () => {
  result = await transform(FIXTURE, {
    filename: 'test.astro',
  });
});

test('Astro style imports placed after frontmatter imports', () => {
  const idx1 = result.code.indexOf('../styles/global.css');
  const idx2 = result.code.indexOf('test.astro?astro&type=style&index=0&lang.css');
  assert.ok(idx2 > idx1);
});

test.run();
