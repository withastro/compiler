import { transform } from '@astrojs/compiler';
import { test } from 'uvu';
import * as assert from 'uvu/assert';

const FIXTURE = '<div>Hello world!</div>';

let result: unknown;
test.before(async () => {
	result = await transform(FIXTURE, {
		filename: '/src/components/Cool.astro',
	});
});

test('exports named component', () => {
	assert.match(result.code, 'export default $$Cool', 'Expected output to contain named export');
});

test.run();
