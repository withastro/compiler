import { type TransformResult, transform } from '@astrojs/compiler';
import { test } from 'uvu';
import * as assert from 'uvu/assert';
import { preprocessStyle } from '../utils.js';

const FIXTURE = `
---
let value = 'world';
---

<style lang="scss"></style>

<div>Hello world!</div>

<div>Ahhh</div>
`;

let result: TransformResult;
test.before(async () => {
	result = await transform(FIXTURE, {
		sourcemap: true,
		preprocessStyle,
	});
});

test('can compile empty style', () => {
	assert.ok(result.code, 'Expected to compile with empty style.');
});

test.run();
