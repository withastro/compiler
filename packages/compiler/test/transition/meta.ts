import { type TransformResult, transform } from '@astrojs/compiler';
import { describe, it, before } from 'node:test';
import assert from 'node:assert/strict';

const FIXTURE = `
<div transition:animate="slide"></div>
`;

let result: TransformResult;

describe('transition/meta', () => {
	before(async () => {
		result = await transform(FIXTURE, {
			resolvePath: async (s) => s,
		});
	});

	it('tagged with propagation metadata', () => {
		assert.deepStrictEqual(result.propagation, true);
	});
});
