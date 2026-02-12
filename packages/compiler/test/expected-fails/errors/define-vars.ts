import { transform } from '@astrojs/compiler';
import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

describe('define-vars', { skip: true }, () => {
	it('define:vars warning', async () => {
		const result = await transform(
			`<Fragment><slot /></Fragment>
<style define:vars={{ color: 'red' }}></style>`,
			{ filename: '/src/components/Foo.astro' },
		);
		assert.ok(Array.isArray(result.diagnostics));
		assert.strictEqual(result.diagnostics.length, 1);
		assert.strictEqual(result.diagnostics[0].code, 2007);
	});

	it('define:vars no warning', async () => {
		const result = await transform(
			`<div><slot /></div>
<style define:vars={{ color: 'red' }}></style>`,
			{ filename: '/src/components/Foo.astro' },
		);
		assert.ok(Array.isArray(result.diagnostics));
		assert.strictEqual(result.diagnostics.length, 0);
	});
});
