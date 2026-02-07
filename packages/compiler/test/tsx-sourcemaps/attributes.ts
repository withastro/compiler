import { describe, it } from 'node:test';
import assert from 'node:assert/strict';
import { testTSXSourcemap } from '../utils.js';

describe('tsx-sourcemaps/attributes', { skip: true }, () => {
	it('shorthand attribute', async () => {
		const input = '<div {name} />';

		const output = await testTSXSourcemap(input, 'name');
		assert.deepStrictEqual(output, {
			source: 'index.astro',
			line: 1,
			column: 6,
			name: null,
		});
	});

	it('empty quoted attribute', async () => {
		const input = `<div src="" />`;

		const open = await testTSXSourcemap(input, '"');
		assert.deepStrictEqual(open, {
			source: 'index.astro',
			line: 1,
			column: 9,
			name: null,
		});
	});

	it('template literal attribute', async () => {
		const input = `---
---
<Tag src=\`bar\${foo}\` />`;

		const open = await testTSXSourcemap(input, 'foo');
		assert.deepStrictEqual(open, {
			source: 'index.astro',
			line: 3,
			column: 16,
			name: null,
		});
	});

	it('multiline quoted attribute', async () => {
		const input = `<path d="M 0
C100 0
Z" />`;

		const output = await testTSXSourcemap(input, 'Z');
		assert.deepStrictEqual(output, {
			source: 'index.astro',
			line: 3,
			column: 1,
			name: null,
		});
	});
});
