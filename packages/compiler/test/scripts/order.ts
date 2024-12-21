import { transform } from '@astrojs/compiler';
import { test } from 'uvu';
import * as assert from 'uvu/assert';

test('outputs scripts in expected order', async () => {
	const result = await transform(`
    <script type="module">console.log(1)</script>
    <script type="module">console.log(2)</script>`);
	const matches = result.code.match(/console\.log\((\d)\)/g);

	if (!matches) throw new Error('No matches');

	assert.match(matches[0], '1');
	assert.match(matches[1], '2');
});

test.run();
