import { type TransformResult, transform } from '@astrojs/compiler';
import { describe, it, before } from 'node:test';
import assert from 'node:assert/strict';
import { preprocessStyle } from '../../utils.js';

const FIXTURE = `
---
let value = 'world';
---

<style lang="scss"></style>

<div>Hello world!</div>

<div>Ahhh</div>
`;

describe('styles/empty-style', { skip: true }, () => {
	let result: TransformResult;
	before(async () => {
		result = await transform(FIXTURE, {
			sourcemap: true,
			preprocessStyle,
		});
	});

	it('can compile empty style', () => {
		assert.ok(result.code, 'Expected to compile with empty style.');
	});
});
