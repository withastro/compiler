import { type TransformResult, transform } from '@astrojs/compiler';
import { describe, it, before } from 'node:test';
import assert from 'node:assert/strict';

const FIXTURE = `
---
---
<style>
    .thing { color: green; }
    .url-space { background: url('/white space.png'); }
    .escape:not(#\\#) { color: red; }
</style>
`;

describe('static-extraction/css', { skip: true }, () => {
	let result: TransformResult;
	before(async () => {
		result = await transform(FIXTURE);
	});

	it('extracts styles', () => {
		assert.deepStrictEqual(
			result.css.length,
			1,
			`Incorrect CSS returned. Expected a length of 1 and got ${result.css.length}`,
		);
	});

	it('escape url with space', () => {
		assert.ok(result.css[0].includes('background:url(/white\\ space.png)'));
	});

	it('escape css syntax', () => {
		assert.ok(result.css[0].includes(':not(#\\#)'));
	});
});
