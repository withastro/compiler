import { type TransformResult, transform } from '@astrojs/compiler';
import { test } from 'uvu';
import * as assert from 'uvu/assert';

test('define:vars warning', async () => {
	const result = await transform(
		`<Fragment><slot /></Fragment>
<style define:vars={{ color: 'red' }}></style>`,
		{ filename: '/src/components/Foo.astro' }
	);
	assert.ok(Array.isArray(result.diagnostics));
	assert.is(result.diagnostics.length, 1);
	assert.is(result.diagnostics[0].code, 2007);
});

test('define:vars no warning', async () => {
	const result = await transform(
		`<div><slot /></div>
<style define:vars={{ color: 'red' }}></style>`,
		{ filename: '/src/components/Foo.astro' }
	);
	assert.ok(Array.isArray(result.diagnostics));
	assert.is(result.diagnostics.length, 0);
});

test.run();
