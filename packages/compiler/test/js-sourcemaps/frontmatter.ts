import { test } from 'uvu';
import * as assert from 'uvu/assert';
import { testJsSourcemap } from '../utils';

test('frontmatter', async () => {
  const input = `---
nonexistent
---
`;
  const output = await testJsSourcemap(input, 'nonexistent');

  assert.equal(output, {
    line: 2,
    column: 1,
    source: 'index.astro',
    name: null,
  });
});

test.run();
