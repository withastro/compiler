import { transform } from '@astrojs/compiler';
import { test } from 'uvu';
import * as assert from 'uvu/assert';

const FIXTURE = `
---
import Parent from './Parent.astro';
---
<Parent>
  <div></div>
</Parent>
`;

let result: unknown;
test.before(async () => {
	result = await transform(FIXTURE, {
		resolvePath: async (s) => s,
		resultScopedSlot: true,
	});
});

test('resultScopedSlot: includes the result object in the call to the slot', () => {
	assert.match(result.code, /\(\$\$result\) =>/);
});

test.run();
