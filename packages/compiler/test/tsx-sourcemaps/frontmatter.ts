import { test } from 'uvu';
import * as assert from 'uvu/assert';
import { testSourcemap } from '../utils';

test('frontmatter', async () => {
  const input = `---
nonexistent
---
`;
  const output = await testSourcemap(input, 'nonexistent');

  assert.equal(output, {
    line: 2,
    column: 1,
    source: 'index.astro',
    name: null,
  });
});

test.run();
