import { type TransformResult, transform } from '@astrojs/compiler';
import { before, describe, it } from 'node:test';
import assert from 'node:assert/strict';

const FIXTURE = `
<slot />
`;

describe('head-metadata/without-head', () => {
	let result: TransformResult;
	before(async () => {
		result = await transform(FIXTURE, {
			filename: 'test.astro',
		});
	});

	it('containsHead is false', () => {
		assert.deepStrictEqual(result.containsHead, false);
	});
});
