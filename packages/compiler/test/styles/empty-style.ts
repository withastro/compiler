import { type TransformResult, transform, preprocessStyles } from '@astrojs/compiler-rs';
import { describe, it, before } from 'node:test';
import assert from 'node:assert/strict';
import { preprocessStyle } from '../utils.js';

const FIXTURE = `
---
let value = 'world';
---

<style lang="scss"></style>

<div>Hello world!</div>

<div>Ahhh</div>
`;

describe('styles/empty-style', () => {
	let result: TransformResult;
	before(async () => {
		const preprocessedStyles = await preprocessStyles(FIXTURE, preprocessStyle);
		result = transform(FIXTURE, {
			sourcemap: 'external',
			preprocessedStyles,
		});
	});

	it('can compile empty style', () => {
		assert.ok(result.code, 'Expected to compile with empty style.');
	});
});
