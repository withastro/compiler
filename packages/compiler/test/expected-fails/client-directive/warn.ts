import { type TransformResult, transform } from '@astrojs/compiler';
import assert from 'node:assert/strict';
import { before, describe, it } from 'node:test';

const FIXTURE = `
<script client:load></script>
`;

describe('client-directive/warn', { skip: true }, () => {
	let result: TransformResult;
	before(async () => {
		result = await transform(FIXTURE);
	});

	it('reports a warning for using a client directive', () => {
		assert.ok(Array.isArray(result.diagnostics));
		assert.strictEqual(result.diagnostics.length, 2);
		assert.deepStrictEqual(result.diagnostics[0].severity, 2);
		assert.ok(result.diagnostics[0].text.includes('does not need the client:load directive'));
	});
});
