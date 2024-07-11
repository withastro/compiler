import { convertToTSX } from '@astrojs/compiler';
import { test } from 'uvu';
import * as assert from 'uvu/assert';

test('404 generates a valid identifier', async () => {
	const input = '<div {name} />';

	const output = await convertToTSX(input, { filename: '404.astro', sourcemap: 'inline' });
	assert.match(output.code, 'export default function __AstroComponent_');
});
