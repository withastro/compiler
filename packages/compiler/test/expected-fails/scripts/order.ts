import { transform } from '@astrojs/compiler';
import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

describe('scripts/order', { skip: true }, () => {
	it('outputs scripts in expected order', async () => {
		const result = await transform(
			`
    <script>console.log(1)</script>
    <script>console.log(2)</script>`,
			{
				experimentalScriptOrder: true,
			}
		);

		const scripts = result.scripts;

		if (scripts[0].type === 'external') throw new Error('Script is external');
		if (scripts[1].type === 'external') throw new Error('Script is external');

		assert.ok(scripts[0].code.includes('console.log(1)'));
		assert.ok(scripts[1].code.includes('console.log(2)'));
	});
});
