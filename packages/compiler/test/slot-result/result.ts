import { type TransformResult, transform } from '@astrojs/compiler';
import { describe, it, before } from 'node:test';
import assert from 'node:assert/strict';

const FIXTURE = `
---
import Parent from './Parent.astro';
---
<Parent>
  <div></div>
</Parent>
`;

let result: TransformResult;

describe('slot-result/result', () => {
	before(async () => {
		result = await transform(FIXTURE, {
			resolvePath: async (s) => s,
			resultScopedSlot: true,
		});
	});

	it('resultScopedSlot: includes the result object in the call to the slot', () => {
		assert.match(result.code, /\(\$\$result\) =>/);
	});
});
