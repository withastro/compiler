import { convertToTSX } from '@astrojs/compiler';
import { test } from 'uvu';
import * as assert from 'uvu/assert';

const FIXTURE = '<div class={';

let result: unknown;
test.before(async () => {
	result = await convertToTSX(FIXTURE, {
		filename: '/src/components/unfinished.astro',
	});
});

test('did not crash on unfinished component', () => {
	assert.ok(result);
	assert.ok(Array.isArray(result.diagnostics));
	assert.is(result.diagnostics.length, 0);
});

test.run();
