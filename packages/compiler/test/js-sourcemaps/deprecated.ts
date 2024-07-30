import { test } from 'uvu';
import * as assert from 'uvu/assert';
import { testJSSourcemap } from '../utils.js';

test('script is:inline', async () => {
	const input = `---
    /** @deprecated */
const deprecated = "Astro"
deprecated;
const hello = "Astro"
---
`;
	const output = await testJSSourcemap(input, 'deprecated;');

	assert.equal(output, {
		line: 4,
		column: 1,
		source: 'index.astro',
		name: null,
	});
});

test.run();
