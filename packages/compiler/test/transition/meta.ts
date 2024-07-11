import { transform } from '@astrojs/compiler';
import { test } from 'uvu';
import * as assert from 'uvu/assert';

const FIXTURE = `
<div transition:animate="slide"></div>
`;

let result: unknown;
test.before(async () => {
	result = await transform(FIXTURE, {
		resolvePath: async (s) => s,
	});
});

test('tagged with propagation metadata', () => {
	assert.equal(result.propagation, true);
});

test.run();
