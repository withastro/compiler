import { test } from 'uvu';
import * as assert from 'uvu/assert';
import { transform } from '@astrojs/compiler';

const FIXTURE = `
---
import Parent from './Parent.astro';
---
<Parent>
  <div></div>
</Parent>
`;

let result;
test.before(async () => {
  result = await transform(FIXTURE, {
    resolvePath: async (s) => s,
    resultScopedSlot: true,
  });
});

test('resultScopedSlot: includes the result object in the call to the slot', () => {
  assert.match(result.code, new RegExp(`\\(\\$\\$result\\) =>`));
});

test.run();
