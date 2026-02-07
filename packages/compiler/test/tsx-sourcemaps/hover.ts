import { describe, it } from 'node:test';
import assert from 'node:assert/strict';
import { testTSXSourcemap } from '../utils.js';

const fixture = `---
    const MyVariable = "Astro"

    /** Documentation */
    const MyDocumentedVariable = "Astro"

    /** @author Astro */
    const MyJSDocVariable = "Astro"
---
`;

describe('tsx-sourcemaps/hover', { skip: true }, () => {
	it('hover I', async () => {
		const input = fixture;
		const output = await testTSXSourcemap(input, 'MyVariable');

		assert.deepStrictEqual(output, {
			line: 2,
			column: 11,
			source: 'index.astro',
			name: null,
		});
	});

	it('hover II', async () => {
		const input = fixture;
		const output = await testTSXSourcemap(input, 'MyDocumentedVariable');

		assert.deepStrictEqual(output, {
			line: 5,
			column: 11,
			source: 'index.astro',
			name: null,
		});
	});

	it('hover III', async () => {
		const input = fixture;
		const output = await testTSXSourcemap(input, 'MyJSDocVariable');

		assert.deepStrictEqual(output, {
			line: 8,
			column: 11,
			source: 'index.astro',
			name: null,
		});
	});
});
