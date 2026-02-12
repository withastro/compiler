import { convertToTSX } from '@astrojs/compiler';
import { describe, it } from 'node:test';
import assert from 'node:assert/strict';

describe('tsx-sourcemaps/unfinished-literal', { skip: true }, () => {
	it('does not panic on unfinished template literal attribute', async () => {
		const input = `<div class=\`></div>
  `;
		let error = 0;
		try {
			const output = await convertToTSX(input, { filename: 'index.astro', sourcemap: 'inline' });
			assert.ok(output.code.includes('class={``}'));
		} catch (_e) {
			error = 1;
		}

		assert.deepStrictEqual(error, 0, 'compiler should not have panicked');
	});

	it('does not panic on unfinished double quoted attribute', async () => {
		const input = `<main id="gotcha />`;
		let error = 0;
		try {
			const output = await convertToTSX(input, { filename: 'index.astro', sourcemap: 'inline' });
			assert.ok(output.code.includes(`id="gotcha"`));
		} catch (_e) {
			error = 1;
		}

		assert.deepStrictEqual(error, 0, 'compiler should not have panicked');
	});

	it('does not panic on unfinished single quoted attribute', async () => {
		const input = `<main id='gotcha/>`;
		let error = 0;
		try {
			const output = await convertToTSX(input, { filename: 'index.astro', sourcemap: 'inline' });
			assert.ok(output.code.includes(`id="gotcha"`));
		} catch (_e) {
			error = 1;
		}

		assert.deepStrictEqual(error, 0, 'compiler should not have panicked');
	});
});
