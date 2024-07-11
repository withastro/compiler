import { type TransformResult, transform } from '@astrojs/compiler';
import { test } from 'uvu';
import * as assert from 'uvu/assert';

test('...spread has warning', async () => {
	const result = await transform('<Head ...seo />', { filename: '/src/components/Foo.astro' });
	assert.ok(Array.isArray(result.diagnostics));
	assert.is(result.diagnostics.length, 1);
	assert.is(result.diagnostics[0].code, 2008);
});

test('{...spread} does not have warning', async () => {
	const result = await transform('<Head {...seo} />', { filename: '/src/components/Foo.astro' });
	assert.ok(Array.isArray(result.diagnostics));
	assert.is(result.diagnostics.length, 0);
});

test.run();
