import { convertToTSX } from '@astrojs/compiler-rs';
import { describe, it } from 'node:test';
import assert from 'node:assert/strict';

describe('tsx-sourcemaps/404', { skip: true }, () => {
	it('404 generates a valid identifier', async () => {
		const input = '<div {name} />';

		const output = await convertToTSX(input, { filename: '404.astro', sourcemap: 'inline' });
		assert.ok(output.code.includes('export default function __AstroComponent_'));
	});
});
