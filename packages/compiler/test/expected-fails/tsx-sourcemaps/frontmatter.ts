import { describe, it } from 'node:test';
import assert from 'node:assert/strict';
import { testTSXSourcemap } from '../../utils.js';

describe('tsx-sourcemaps/frontmatter', { skip: true }, () => {
	it('frontmatter', async () => {
		const input = `---
nonexistent
---
`;
		const output = await testTSXSourcemap(input, 'nonexistent');

		assert.deepStrictEqual(output, {
			line: 2,
			column: 1,
			source: 'index.astro',
			name: null,
		});
	});
});
