import { parse, teardown } from '@astrojs/compiler';
import { describe, it } from 'node:test';
import assert from 'node:assert/strict';

const FIXTURE = '<div>hello</div>';

describe('teardown/parse', { skip: true }, () => {
	it('parse still works after teardown', async () => {
		const ast1 = await parse(FIXTURE);
		assert.ok(ast1);
		teardown();
		// Make sure `parse` creates a new WASM instance after teardown removed the previous one
		const ast2 = await parse(FIXTURE);
		assert.ok(ast2);
	});
});
