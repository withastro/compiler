import { transform } from '@astrojs/compiler';
import { test } from 'uvu';
import * as assert from 'uvu/assert';

test('outputs scripts in expected order', async () => {
	const result = await transform(`
    <script>console.log(1)</script>
    <script>console.log(2)</script>`);

	const scripts = result.scripts;

	// for typescript
	if (scripts[0].type === 'external') throw new Error('Script is external');
	if (scripts[1].type === 'external') throw new Error('Script is external');

	assert.match(scripts[0].code, 'console.log(1)');
	assert.match(scripts[1].code, 'console.log(2)');
});

test.run();
