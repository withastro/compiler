import { transform } from '@astrojs/compiler';
import { test } from 'uvu';
import * as assert from 'uvu/assert';

const FIXTURE = '<html><head><title>Ah</title></head></html>';

let result: unknown;
test.before(async () => {
	result = await transform(FIXTURE);
});

test('head injection', () => {
	assert.match(
		result.code,
		'$$renderHead($$result)',
		'Expected output to contain $$renderHead($$result) injection point'
	);
});

test.run();
