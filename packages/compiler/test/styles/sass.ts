import { type TransformResult, transform } from '@astrojs/compiler';
import { test } from 'uvu';
import * as assert from 'uvu/assert';
import { preprocessStyle } from '../utils.js';

const FIXTURE = `
---
let value = 'world';
---

<style lang="scss" define:vars={{ a: 0 }}>
$color: red;

div {
  color: $color;
}
</style>

<div>Hello world!</div>

<div>Ahhh</div>

<style lang="scss">
$color: green;
div {
  color: $color;
}
</style>
`;

let result: TransformResult;
test.before(async () => {
	result = await transform(FIXTURE, {
		sourcemap: true,
		preprocessStyle,
	});
});

test('transforms scss one', () => {
	assert.match(result.css[0], 'color:red', 'Expected "color:red" to be present.');
});

test('transforms scss two', () => {
	assert.match(
		result.css[result.css.length - 1],
		'color:green',
		'Expected "color:green" to be present.'
	);
});

test.run();
