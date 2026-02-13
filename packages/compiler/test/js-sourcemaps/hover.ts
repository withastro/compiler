import { describe, it } from 'node:test';
import assert from 'node:assert/strict';
import { testJSSourcemap } from '../utils.js';

const fixture = `---
    const MyVariable = "Astro"

    /** Documentation */
    const MyDocumentedVariable = "Astro"

    /** @author Astro */
    const MyJSDocVariable = "Astro"
---
`;

describe('js-sourcemaps/hover', () => {
	it('hover I', async () => {
		const input = fixture;
		const output = await testJSSourcemap(input, 'MyVariable');

		assert.deepStrictEqual(output, {
			source: 'index.astro',
			line: 2,
			column: 4,
			name: null,
		});
	});

	it('hover II', async () => {
		const input = fixture;
		const output = await testJSSourcemap(input, 'MyDocumentedVariable');

		assert.deepStrictEqual(output, {
			source: 'index.astro',
			line: 5,
			column: 4,
			name: null,
		});
	});

	it('hover III', async () => {
		const input = fixture;
		const output = await testJSSourcemap(input, 'MyJSDocVariable');

		assert.deepStrictEqual(output, {
			source: 'index.astro',
			line: 8,
			column: 4,
			name: null,
		});
	});
});
