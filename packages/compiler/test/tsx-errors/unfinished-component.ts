import { convertToTSX } from '@astrojs/compiler';
import { describe, it, before } from 'node:test';
import assert from 'node:assert/strict';
import type { TSXResult } from '../../types.js';

const FIXTURE = '<div class={';

describe('tsx-errors/unfinished-component', { skip: true }, () => {
	let result: TSXResult;
	before(async () => {
		result = await convertToTSX(FIXTURE, {
			filename: '/src/components/unfinished.astro',
		});
	});

	it('did not crash on unfinished component', () => {
		assert.ok(result);
		assert.ok(Array.isArray(result.diagnostics));
		assert.strictEqual(result.diagnostics.length, 0);
	});
});
