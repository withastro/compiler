import { describe, it } from 'node:test';
import assert from 'node:assert/strict';
import { testJSSourcemap } from '../utils.js';

describe('js-sourcemaps/deprecated', () => {
	it('script is:inline', async () => {
		const input = `---
    /** @deprecated */
const deprecated = "Astro"
deprecated;
const hello = "Astro"
---
`;
		const output = await testJSSourcemap(input, 'deprecated;');

		assert.deepStrictEqual(output, {
			source: 'index.astro',
			line: 4,
			column: 0,
			name: null,
		});
	});
});
