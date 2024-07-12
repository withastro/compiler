import { transform } from '@astrojs/compiler';
import { test } from 'uvu';
import * as assert from 'uvu/assert';

const FIXTURE = `
<style>
  body { color: green; }
</style>
`;

let result: unknown;
test.before(async () => {
	result = await transform(FIXTURE, {
		filename: 'test.astro',
	});
});

test('Astro style imports are included in the compiled JS', () => {
	const idx = result.code.indexOf('test.astro?astro&type=style&index=0&lang.css');
	assert.not.equal(idx, -1);
});

test.run();
