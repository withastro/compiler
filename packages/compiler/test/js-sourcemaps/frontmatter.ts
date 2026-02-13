import { describe, it } from 'node:test';
import assert from 'node:assert/strict';
import { testJSSourcemap } from '../utils.js';

describe('js-sourcemaps/frontmatter', () => {
	it('frontmatter', async () => {
		const input = `---
nonexistent
---
`;
		const output = await testJSSourcemap(input, 'nonexistent');

		assert.deepStrictEqual(output, {
			source: 'index.astro',
			line: 2,
			column: 0,
			name: null,
		});
	});
});
