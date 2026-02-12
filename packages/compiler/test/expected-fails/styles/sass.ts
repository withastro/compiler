import assert from 'node:assert/strict';
import { before, describe, it } from 'node:test';
import { type TransformResult, transform } from '@astrojs/compiler';
import { preprocessStyle } from '../../utils.js';

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

describe('styles/sass', { skip: true }, () => {
	let result: TransformResult;
	before(async () => {
		result = await transform(FIXTURE, {
			sourcemap: true,
			preprocessStyle,
		});
	});

	it('transforms scss one', () => {
		assert.ok(result.css[0].includes('color:red'), 'Expected "color:red" to be present.');
	});

	it('transforms scss two', () => {
		assert.ok(
			result.css[result.css.length - 1].includes('color:green'),
			'Expected "color:green" to be present.',
		);
	});
});
