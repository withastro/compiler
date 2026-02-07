import { describe, it } from 'node:test';
import assert from 'node:assert/strict';
import { testTSXSourcemap } from '../utils.js';

describe('tsx-sourcemaps/multibyte', { skip: true }, () => {
	it('multibyte content', async () => {
		const input = '<h1>ツ</h1>';

		const output = await testTSXSourcemap(input, 'ツ');
		assert.deepStrictEqual(output, {
			source: 'index.astro',
			line: 1,
			column: 4,
			name: null,
		});
	});

	it('content after multibyte character', async () => {
		const input = '<h1>ツ</h1><p>foobar</p>';

		const output = await testTSXSourcemap(input, 'foobar');
		assert.deepStrictEqual(output, {
			source: 'index.astro',
			line: 1,
			column: 13,
			name: null,
		});
	});

	it('many characters', async () => {
		const input = '<h1>こんにちは</h1>';

		const output = await testTSXSourcemap(input, 'ん');
		assert.deepStrictEqual(output, {
			source: 'index.astro',
			line: 1,
			column: 5,
			name: null,
		});
	});

	it('many characters', async () => {
		const input = '<h1>こんにちは</h1>';

		const output = await testTSXSourcemap(input, 'に');
		assert.deepStrictEqual(output, {
			source: 'index.astro',
			line: 1,
			column: 6,
			name: null,
		});
	});
});
