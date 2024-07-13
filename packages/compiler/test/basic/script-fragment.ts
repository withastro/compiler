import { transform } from '@astrojs/compiler';
import { test } from 'uvu';
import * as assert from 'uvu/assert';

const FIXTURE = `<script src={Astro.resolve("../scripts/no_hoist_nonmodule.js")}></script>`;

let result: unknown;
test.before(async () => {
	result = await transform(FIXTURE);
});

test('script fragment', () => {
	assert.ok(result.code, 'Can compile script fragment');
});

test.run();
