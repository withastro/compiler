import { type TransformResult, transform } from '@astrojs/compiler';
import { describe, it, before } from 'node:test';
import assert from 'node:assert/strict';

const FIXTURE = `
---
let value = 'world';
---

<style>div { color: red; }</style>

<div>Hello world!</div>
`;

let result: TransformResult;

describe('styles/emit-scope', () => {
	before(async () => {
		result = await transform(FIXTURE, {
			sourcemap: true,
		});
	});

	it('emits a scope', () => {
		assert.ok(result.scope, 'Expected to return a scope');
	});
});
